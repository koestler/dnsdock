package main

import (
	"errors"
	"fmt"
	"github.com/koestler/dnsdock/dnsStorage"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/koestler/dnsdock/httpServer"
	"github.com/koestler/dnsdock/resolver"

	dockerapi "github.com/fsouza/go-dockerclient"
)

// is set through linker by build.sh
var buildVersion string
var buildTime string

func getopt(name, def string) string {
	if env := os.Getenv(name); env != "" {
		return env
	}
	return def
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println("github.com/koestler/dnsdock version:", buildVersion)
		fmt.Println("build at:", buildTime)
		os.Exit(0)
	}
	log.Printf("Starting koestler-dnsdock %s ...", buildVersion)

	err := run()
	if err != nil {
		log.Fatal("dnsdock: ", err)
	}
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

	if err := writeDnsmasqd(); err != nil {
		log.Printf("[ERROR] could not write dnsmasq conf: %v", err)
	}

	var hostIP net.IP
	if envHostIP := os.Getenv("HOST_IP"); envHostIP != "" {
		hostIP = net.ParseIP(envHostIP)
		log.Println("using address for --net=host:", hostIP)
	}

	// create dnsStorage
	storage := dnsStorage.NewDnsStorage()

	// dns dnsResolver
	dnsResolver, err := resolver.NewResolver(storage)
	if err != nil {
		return err
	}
	defer dnsResolver.Close()

	localDomain := os.Getenv("LOCAL_DOMAIN")
	if localDomain == "" {
		localDomain = "docker"
	}

	// start http server
	env := &httpServer.Environment{
		Storage: storage,
	}
	httpServer.Run("", 80, env)

	go func() {
		dnsResolver.Wait()
		exitReason <- errors.New("dns resolver exited")
	}()
	go func() {
		exitReason <- registerContainers(docker, nil, dnsResolver, storage, localDomain, hostIP)
	}()

	return <-exitReason
}
