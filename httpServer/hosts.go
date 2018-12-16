package httpServer

import "github.com/koestler/dnsdock/dnsStorage"

type Host struct {
	Name    string
	Address string
	Aliases []string
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
		Name:    host.Name,
		Address: host.Address.String(),
		Aliases: host.Aliases,
	}
}
