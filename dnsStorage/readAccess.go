package dnsStorage

import (
	"github.com/miekg/dns"
	"log"
	"net"
	"strings"
)

func (d *DnsStorage) FindHostAddresses(name string) (addrs []net.IP) {
	log.Printf("dnsStorage: FindHostAddresses name=%s", name)

	d.hostsMutex.RLock()
	defer d.hostsMutex.RUnlock()

	for _, host := range d.hosts {
		log.Printf("dnsStorage dns.Fqdn(host.Name) == name : %s == %s", dns.Fqdn(host.Name), name)
		if dns.Fqdn(host.Name) == name {
			addrs = append(addrs, host.Address)
		}

		for _, alias := range host.Aliases {
			if dns.Fqdn(alias) == name {
				addrs = append(addrs, host.Address)
			}
		}
	}

	log.Printf("dnsStorage: FindHostAddresses return addrs=%v", addrs)
	return
}

func (d *DnsStorage) FindReverseHost(address string) (hosts []string) {
	log.Printf("dnsStorage: FindReverseHost address=%s", address)

	d.hostsMutex.RLock()
	defer d.hostsMutex.RUnlock()

	address = strings.ToLower(dns.Fqdn(address))

	for _, entry := range d.hosts {
		if r, _ := dns.ReverseAddr(entry.Address.String()); address == r {
			hosts = append(hosts, dns.Fqdn(entry.Name))
		}
	}
	return
}

func (d *DnsStorage) GetHosts() (hosts Hosts) {
	log.Printf("dnsStorage: GetHosts")

	d.hostsMutex.RLock()
	defer d.hostsMutex.RUnlock()

	hosts = make(Hosts, len(d.hosts))
	for id, host := range d.hosts {
		hosts[id] = host
	}
	return
}
