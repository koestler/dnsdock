package dnsStorage

import (
	"net"
	"sync"
)

type HostId string

type Host struct {
	Id      HostId
	Address net.IP
	Name    string
	Aliases []string
}

type Hosts map[HostId]Host

type DnsStorage struct {
	// internal hosts
	hosts      Hosts
	hostsMutex sync.RWMutex

	subscriptions []subscription

	// communication channels
	subscribeChannel  chan *subscription
	addHostChannel    chan Host
	removeHostChannel chan HostId
}

type subscription struct {
	onAdd    chan Host
	onRemove chan HostId
}

func (d *DnsStorage) MainRoutine() {
	for {
		select {
		case newSubscription := <-d.subscribeChannel:
			d.subscriptions = append(d.subscriptions, *newSubscription)
		case newHost := <-d.addHostChannel:
			d.handleAddHost(newHost)
		case hostId := <-d.removeHostChannel:
			d.handleRemoveHost(hostId)
		}
	}
}

func (d *DnsStorage) handleAddHost(host Host) {
	// add to hosts list
	d.hostsMutex.Lock()
	_, exists := d.hosts[host.Id]
	if !exists {
		d.hosts[host.Id] = host
		d.hostsMutex.Unlock()
		return
	}
	d.hostsMutex.Unlock()

	// publish to subscribes
	for _, subscription := range d.subscriptions {
		subscription.onAdd <- host
	}
}

func (d *DnsStorage) handleRemoveHost(hostId HostId) {
	d.hostsMutex.Lock()
	_, exists := d.hosts[hostId]
	if !exists {
		delete(d.hosts, hostId)
		d.hostsMutex.Unlock()
		return
	}
	d.hostsMutex.Unlock()

	// publish to subscribes
	for _, subscription := range d.subscriptions {
		subscription.onRemove <- hostId
	}
}
