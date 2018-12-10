package httpServer

import (
	"net/http"
	"encoding/json"
)

func HandleHostsGet(env *Environment, w http.ResponseWriter, r *http.Request) Error {
	hosts := env.DnsResolver.GetHosts()

	writeJsonHeaders(w)
	b, err := json.MarshalIndent(hosts, "", "    ")
	if err != nil {
		return StatusError{500, err}
	}
	w.Write(b)
	return nil
}
