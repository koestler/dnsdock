package dnsStorage

func (d *DnsStorage) MainRoutine() {
	for {
		select {
		case s := <-d.subscribeChannel:
			d.subscriptions[s] = true
		case s := <-d.unsubscribeChannel:
			delete(d.subscriptions, s)
		case newHost := <-d.addHostChannel:
			d.handleAddHost(newHost)
		case hostId := <-d.removeHostChannel:
			d.handleRemoveHost(hostId)
		}
	}
}

func (d *DnsStorage) Subscribe() (s *Subscription) {
	s = &Subscription{
		OnAdd:    make(chan Host, 4),
		OnRemove: make(chan string, 4),
	}
	d.subscribeChannel <- s;
	return
}

func (d *DnsStorage) Unsubscribe(s *Subscription) {
	close(s.OnAdd)
	close(s.OnRemove)
	d.unsubscribeChannel <- s;
}

func (d *DnsStorage) handleAddHost(host Host) {
	// add to hosts list
	d.hostsMutex.Lock()
	_, exists := d.hosts[host.Id]
	if exists {
		d.hostsMutex.Unlock()
		return
	}

	d.hosts[host.Id] = host
	d.hostsMutex.Unlock()

	// publish to subscribes
	for subscription, _ := range d.subscriptions {
		subscription.OnAdd <- host
	}
}

func (d *DnsStorage) handleRemoveHost(hostId string) {
	d.hostsMutex.Lock()
	_, exists := d.hosts[hostId]
	if !exists {
		d.hostsMutex.Unlock()
		return
	}

	delete(d.hosts, hostId)
	d.hostsMutex.Unlock()

	// publish to subscribes
	for subscription, _ := range d.subscriptions {
		subscription.OnRemove <- hostId
	}
}
