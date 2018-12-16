package httpServer

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

func HandleGetHosts(env *Environment, w http.ResponseWriter, r *http.Request) Error {
	response := getAllHosts(env)

	w.Header().Set("Content-Model", "application/json; charset=UTF-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	b, err := json.MarshalIndent(response, "", "    ")
	if err != nil {
		return StatusError{500, err}
	}

	_, err = w.Write(b)
	if err != nil {
		return StatusError{500, err}
	}
	return nil
}

func HandleWsHosts(env *Environment, w http.ResponseWriter, r *http.Request) Error {
	// upgrade to websocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgraded failed: %v", err)
		return nil
	}

	// subscribe to dnsStorage
	subscription := env.Storage.Subscribe()

	// setup close handler
	conn.SetCloseHandler(func(code int, text string) error {
		env.Storage.Unsubscribe(subscription)
		return nil
	})

	// discard incoming messages / close on read error
	go func() {
		for {
			if _, _, err := conn.NextReader(); err != nil {
				if err := conn.Close(); err != nil {
					log.Printf("HandleWsHosts error during close: %v", err)
				}
				break
			}
		}
	}()

	// send add messages for initial state
	for hostId, host := range getAllHosts(env) {
		sendAddMessage(conn, hostId, host)
	}

	go func() {
		for {
			var err error

			select {
			case newHost, ok := <-subscription.OnAdd:
				if !ok {
					return
				}

				if newHost.Address.IsGlobalUnicast() {
					sendAddMessage(conn, newHost.Id, convertHost(newHost))
				}
			case hostId, ok := <-subscription.OnRemove:
				if !ok {
					return
				}

				sendRemoveMessage(conn, hostId)
			}
			if err != nil {
				log.Printf("HandleWsHost: error during conn.WriteJSON: %v", err)
				_ = conn.Close()
			}
		}
	}()

	return nil;
}

type RemoveMessage struct {
	Type   string
	HostId string
}

type AddMessage struct {
	Type   string
	HostId string
	Host   Host
}

func sendAddMessage(conn *websocket.Conn, hostId string, host Host) {
	err := conn.WriteJSON(AddMessage{
		Type:   "add",
		HostId: hostId,
		Host:   host,
	})

	if err != nil {
		log.Printf("sendAddMessage: error during conn.WriteJSON: %v", err)
	}
}

func sendRemoveMessage(conn *websocket.Conn, hostId string) {
	err := conn.WriteJSON(RemoveMessage{
		Type:   "remove",
		HostId: hostId,
	})

	if err != nil {
		log.Printf("sendRemoveMessage: error during conn.WriteJSON: %v", err)
	}
}
