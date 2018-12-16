package dnsStorage

import (
	"net"
	"sync"
)

type Host struct {
	Id      string
	Address net.IP
	Name    string
	Aliases []string
}

type Hosts map[string]Host

type DnsStorage struct {
	// internal hosts
	hosts      Hosts
	hostsMutex sync.RWMutex

	// subscription management
	subscriptions map[*Subscription]bool

	// communication channels
	subscribeChannel   chan *Subscription
	unsubscribeChannel chan *Subscription
	addHostChannel     chan Host
	removeHostChannel  chan string
}

type Subscription struct {
	OnAdd         chan Host
	OnRemove      chan string
}

func NewDnsStorage() (dnsStorage *DnsStorage) {
	dnsStorage = &DnsStorage{
		hosts:              make(Hosts),
		subscriptions:      make(map[*Subscription]bool),
		subscribeChannel:   make(chan *Subscription),
		unsubscribeChannel: make(chan *Subscription),
		addHostChannel:     make(chan Host, 4),
		removeHostChannel:  make(chan string, 16),
	}

	go dnsStorage.MainRoutine()

	return
}

func (d *DnsStorage) AddHost(host Host) {
	d.addHostChannel <- host
}

func (d *DnsStorage) RemoveHost(id string) {
	d.removeHostChannel <- id
}
