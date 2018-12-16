package httpServer

import (
	"github.com/fsouza/go-dockerclient"
	"github.com/koestler/dnsdock/dnsStorage"
	"strconv"
	"time"
)

type Host struct {
	Name      string
	Address   string
	Aliases   []string
	Container Container
	Ports     []Port
}

type Port struct {
	Port     int
	Protocol string
}

type Container struct {
	ID      string         `json:"Id"`
	Created time.Time      `json:"Created,omitempty"`
	Image   string         `json:"Image,omitempty"`
	Mounts  []docker.Mount `json:"Mounts,omitempty"`
}

func getAllHosts(env *Environment) (response map[string]Host) {
	hosts := env.Storage.GetHosts()
	response = make(map[string]Host, len(hosts))

	for id, host := range hosts {
		if !host.Address.IsGlobalUnicast() {
			continue
		}
		response[id] = convertHost(host)
	}
	return
}

func convertHost(host dnsStorage.Host) (Host) {
	return Host{
		Name:      host.Name,
		Address:   host.Address.String(),
		Aliases:   host.Aliases,
		Container: convertContainer(host.Container),
		Ports:     convertPorts(host.Container.NetworkSettings.Ports),
	}
}

func convertContainer(container *docker.Container) (Container) {
	return Container{
		ID:      container.ID,
		Created: container.Created,
		Image:   container.Image,
		Mounts:  container.Mounts,
	}
}

func convertPorts(ports map[docker.Port][]docker.PortBinding) ([]Port) {
	ret := make([]Port, len(ports))

	i := 0
	for port, _ := range ports {
		p, _ := strconv.Atoi(port.Port())
		ret[i] = Port{
			Port:     p,
			Protocol: port.Proto(),
		}
		i += 1
	}

	return ret
}
