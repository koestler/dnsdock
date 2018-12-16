package main

import (
	"errors"
	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/koestler/dnsdock/resolver"
	"log"
	"net"
	"regexp"
	"strings"
)

func registerContainers(
	docker *dockerapi.Client,
	events chan *dockerapi.APIEvents,
	dns resolver.Resolver,
	containerDomain string,
	hostIP net.IP,
) error {
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
