package httpServer

import (
	"encoding/json"
	"net/http"
)

type Host struct {
	Name    string
	Address string
	Aliases []string
}

func HandleHostsGet(env *Environment, w http.ResponseWriter, r *http.Request) Error {
	hosts := env.DnsResolver.GetHosts()

	response := make(map[string]Host, len(hosts))

	for id, host := range hosts {
		if !host.Address.IsGlobalUnicast() {
			continue
		}

		response[id] = Host{
			Name:    host.Names[0],
			Address: host.Address.String(),
			Aliases: host.Names[1:],
		}
	}

	writeJsonHeaders(w)
	b, err := json.MarshalIndent(response, "", "    ")
	if err != nil {
		return StatusError{500, err}
	}
	w.Write(b)
	return nil
}
