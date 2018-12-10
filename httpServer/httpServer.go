package httpServer

import (
	"log"
	"net/http"
	"os"
	"strconv"
)

func Run(bind string, port int, env *Environment) {
	go func() {
		router := newRouter(os.Stdout, env)
		address := bind + ":" + strconv.Itoa(port)

		log.Printf("httpServer: listening on %v", address)
		log.Fatal(router, http.ListenAndServe(address, router))
	}()
}
