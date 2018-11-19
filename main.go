package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/koestler/dnsdock/resolver"

	dockerapi "github.com/fsouza/go-dockerclient"
)

var Version string

func getopt(name, def string) string {
	if env := os.Getenv(name); env != "" {
		return env
	}
	return def
}

func ipAddress() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && !ipnet.IP.IsMulticast() {
			if ipv4 := ipnet.IP.To4(); ipv4 != nil {
				return ipv4.String(), nil
			}
		}
	}

	return "", errors.New("no addresses found")
}

func parseContainerEnv(containerEnv []string, prefix string) map[string]string {
	parsed := make(map[string]string)

	for _, env := range containerEnv {
		if !strings.HasPrefix(env, prefix) {
			continue
		}
		keyVal := strings.SplitN(env, "=", 2)
		if len(keyVal) > 1 {
			parsed[keyVal[0]] = keyVal[1]
		} else {
			parsed[keyVal[0]] = ""
		}
	}

	return parsed
}

func registerContainers(docker *dockerapi.Client, events chan *dockerapi.APIEvents, dns resolver.Resolver, containerDomain string, hostIP net.IP) error {
	// TODO add an options struct instead of passing all as parameters
	// though passing the events channel from an options struct was triggering
	// data race warnings within AddEventListener, so needs more investigation

	if events == nil {
		events = make(chan *dockerapi.APIEvents)
	}
	if err := docker.AddEventListener(events); err != nil {
		return err
	}

	if !strings.HasPrefix(containerDomain, ".") {
		containerDomain = "." + containerDomain
	}

	addContainer := func(containerId string) error {
		container, err := docker.InspectContainer(containerId)
		if err != nil {
			return err
		}

		log.Printf("add container (name=%v, id=%v)", container.Name, containerId)

		first := true

		// register a hostname for each network of this container
		for netId, network := range container.NetworkSettings.Networks {
			log.Printf("  found network (name=%v)", netId)

			// build an unique container name by concatenating the network and the container name
			containerNetName := netId + "_" + strings.Trim(container.Name, "/_")

			// explode this unique string by _, reverse order, append docker and
			// implode using . (dcprojet_somenet -> somenet.dcproject.docker)
			// during this:
			// - ignore "default" and "bridge" as part of the domain
			// - skip duplicate string a.a.b -> a.b
			domainParts := []string{}
			var lastP string
			for _, p := range strings.Split(containerNetName, "_") {
				// - ignore "default" as part of the domain
				if strings.Compare(p, "default") == 0 {
					continue
				}

				// - ignore "bridge" as part of the domain
				if strings.Compare(p, "bridge") == 0 {
					continue
				}

				// - skip duplicate string a.a.b -> a.b
				if strings.Compare(p, lastP) == 0 {
					continue
				}

				domainParts = append([]string{p}, domainParts...)
				lastP = p
			}

			// add .docker at the end
			domainParts = append(domainParts, "docker")
			domain := strings.Join(domainParts, ".")

			// generate aliases
			aliases := make([]string, 0, 5)

			// docker-compose uses the following naming scheme:
			// v1.13.0 : <project>_<service>_<index>_<slug> (-> case A)
			// before  : <project>_<service>_<index>        (-> case B)

			// case B: remove only index
			// if this succeeds, use the version w/o 1. as domain an register the one with 1. as alias
			if len(domainParts) > 0 && strings.Compare(domainParts[0], "1") == 0 {
				aliases = append(aliases, domain)
				domain = strings.Join(domainParts[1:], ".")
			}

			// case A: remove slug and index
			// if this succeeds, use the version w/o slug/index. as domain an register the one with index and slug as alias
			if rHex, _ := regexp.Compile("^[0-9a-f]{2,}$"); len(domainParts) > 0 &&
				strings.Compare(domainParts[1], "1") == 0 &&
				rHex.Match([]byte(domainParts[0])) {

				aliases = append(aliases, domain)
				aliases = append(aliases, strings.Join(domainParts[1:], "."))
				domain = strings.Join(domainParts[2:], ".")
			}

			// for first network only: generate alias by the first 12 characters of the containerId
			if first {
				aliases = append(aliases, containerId[:12]+".docker")
				first = false
			}

			log.Printf("  --> add records (ip='%v', domain='%v', aliases=%v)", network.IPAddress, domain, aliases)

			addr := net.ParseIP(network.IPAddress)
			err = dns.AddHost(containerId+"_"+netId, addr, domain, aliases...)
			if err != nil {
				return err
			}
		}

		return nil
	}

	removeContainer := func(containerId string) error {
		container, err := docker.InspectContainer(containerId)
		if err != nil {
			return err
		}

		for netId, _ := range container.NetworkSettings.Networks {
			dns.RemoveHost(containerId + "_" + netId)
		}

		log.Printf("remove container (name=%v, id=%v)", container.Name, containerId)

		return nil
	}

	containers, err := docker.ListContainers(dockerapi.ListContainersOptions{})
	if err != nil {
		return err
	}

	// add existing containers
	for _, listing := range containers {
		if err := addContainer(listing.ID); err != nil {
			log.Printf("error adding container %s: %s\n", listing.ID[:12], err)
		}
	}

	if err = dns.Listen(); err != nil {
		return err
	}
	defer dns.Close()

	// handle docker api events
	for msg := range events {
		go func(msg *dockerapi.APIEvents) {
			switch msg.Status {
			case "start":
				if err := addContainer(msg.ID); err != nil {
					log.Printf("error adding container %s: %s\n", msg.ID[:12], err)
				}
			case "die":
				if err := removeContainer(msg.ID); err != nil {
					log.Printf("error adding container %s: %s\n", msg.ID[:12], err)
				}
			}
		}(msg)
	}

	return errors.New("docker event loop closed")
}

func writeDnsmasqd(ipAddress string) error {

	filepath := "/etc/dnsmasq.d/dnsdock"

	log.Printf("write dnsmasq configurtion to: %s", filepath)

	// generate configuration file
	conf := make([]string, 0, 17)

	// add forward dns for *.docker
	conf = append(conf, fmt.Sprintf("server=/docker/%s", ipAddress))

	// add reverse dns for 172.16.0.0/12
	for i := 16; i < 32; i++ {
		conf = append(conf, fmt.Sprintf("server=/%d.172.in-addr.arpa/%s", i, ipAddress))
	}

	// write configuration file
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString(strings.Join(conf, "\n") + "\n")

	return nil
}

func run() error {
	// set up the signal handler first to ensure cleanup is handled if a signal is
	// caught while initializing
	exitReason := make(chan error)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		sig := <-c
		log.Println("exit requested by signal:", sig)
		exitReason <- nil
	}()

	docker, err := dockerapi.NewClient(getopt("DOCKER_HOST", "unix:///var/run/docker.sock"))
	if err != nil {
		return err
	}

	address, err := ipAddress()
	if err != nil {
		return err
	}
	log.Println("got local address:", address)

	if err := writeDnsmasqd(address); err != nil {
		log.Printf("[ERROR] could not write dnsmasq conf: %v", err)
	}

	for name, conf := range resolver.HostResolverConfigs.All() {
		err := conf.StoreAddress(address)
		if err != nil {
			log.Printf("[ERROR] error in %s: %s", name, err)
		}
		defer conf.Clean()
	}

	var hostIP net.IP
	if envHostIP := os.Getenv("HOST_IP"); envHostIP != "" {
		hostIP = net.ParseIP(envHostIP)
		log.Println("using address for --net=host:", hostIP)
	}

	dnsResolver, err := resolver.NewResolver()
	if err != nil {
		return err
	}
	defer dnsResolver.Close()

	localDomain := os.Getenv("LOCAL_DOMAIN")
	if localDomain == "" {
		localDomain = "docker"
	}

	go func() {
		dnsResolver.Wait()
		exitReason <- errors.New("dns resolver exited")
	}()
	go func() {
		exitReason <- registerContainers(docker, nil, dnsResolver, localDomain, hostIP)
	}()

	return <-exitReason
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(Version)
		os.Exit(0)
	}
	log.Printf("Starting koestler-dnsdock %s ...", Version)

	err := run()
	if err != nil {
		log.Fatal("dnsdock: ", err)
	}
}
