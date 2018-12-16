package httpServer

import (
	"github.com/koestler/dnsdock/dnsStorage"
	"log"
	"net/http"
)

type Environment struct {
	Storage *dnsStorage.DnsStorage
}

// Error represents a handler error. It provides methods for a HTTP status
// code and embeds the built-in error interface.
type Error interface {
	error
	Status() int
}

// StatusError represents an error with an associated HTTP status code.
type StatusError struct {
	Code int
	Err  error
}

func (statusError StatusError) Error() string {
	return statusError.Err.Error()
}

func (statusError StatusError) Status() int {
	return statusError.Code
}

type HandlerHandleFunc func(e *Environment, w http.ResponseWriter, r *http.Request) Error

type Handler struct {
	Env    *Environment
	Handle HandlerHandleFunc
}

func (handler Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := handler.Handle(handler.Env, w, r)

	if err != nil {
		log.Printf("ServeHTTP err=%v", err)

		switch e := err.(type) {
		case Error:
			// We can retrieve the status here and write out a specific
			// HTTP status code.
			log.Printf("HTTP %d - %s", e.Status(), e)
			http.Error(w, http.StatusText(e.Status()), e.Status())
			return
		default:
			// Any error types we don't specifically look out for default
			// to serving a HTTP 500
			http.Error(w, http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError)
			return
		}
	}
}
