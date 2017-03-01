package resolver

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

type Resolver interface {
	AddHost(id string, addr net.IP, name string, aliases ...string) error
	RemoveHost(id string) error

	Listen() error
	Close()
}

type hostsEntry struct {
	Address net.IP
	Names   []string
}

type serversEntry struct {
	Address net.IP
	Port    int
	Domains []string
}

type dnsResolver struct {
	hostMutex sync.RWMutex

	Port    int
	hosts   map[string]*hostsEntry
	server  *dns.Server
	stopped chan struct{}
}

func NewResolver() (*dnsResolver, error) {
	return &dnsResolver{
		Port:    53,
		hosts:   make(map[string]*hostsEntry),
		stopped: make(chan struct{}),
	}, nil
}

func (r *dnsResolver) AddHost(id string, addr net.IP, name string, aliases ...string) error {
	r.hostMutex.Lock()
	defer r.hostMutex.Unlock()

	names := append([]string{name}, aliases...)

	r.hosts[id] = &hostsEntry{Address: addr, Names: names}
	return nil
}

func (r *dnsResolver) RemoveHost(id string) error {
	r.hostMutex.Lock()
	defer r.hostMutex.Unlock()

	delete(r.hosts, id)
	return nil
}

func (r *dnsResolver) Listen() error {
	addr := fmt.Sprintf(":%d", r.Port)

	listenAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp4", listenAddr)
	if err != nil {
		return err
	}

	r.Port = conn.LocalAddr().(*net.UDPAddr).Port

	startupError := make(chan error)
	r.server = &dns.Server{Handler: r, PacketConn: conn, NotifyStartedFunc: func() {
		startupError <- nil
	}}

	go func() {
		select {
		case startupError <- r.run():
		default:
		}
	}()

	return <-startupError
}

func (r *dnsResolver) run() error {
	defer close(r.stopped)
	return r.server.ActivateAndServe()
}

func (r *dnsResolver) Wait() error {
	<-r.stopped
	return nil
}

func (r *dnsResolver) Close() {
	if r.server != nil {
		r.server.Shutdown()
	}
}

func (r *dnsResolver) ServeDNS(w dns.ResponseWriter, query *dns.Msg) {
	response, err := r.responseForQuery(query)
	if err != nil {
		log.Printf("response error: %T %s", err, err)
		return
	}
	if response == nil {
		return
	}

	err = w.WriteMsg(response)
	if err != nil {
		log.Println("write error:", err)
	}
}

func (r *dnsResolver) responseForQuery(query *dns.Msg) (*dns.Msg, error) {
	// TODO multiple queries?
	name := query.Question[0].Name

	if query.Question[0].Qtype == dns.TypeA {
		if addrs := r.findHost(name); len(addrs) > 0 {
			return dnsAddressRecord(query, name, addrs), nil
		}
	} else if query.Question[0].Qtype == dns.TypePTR {
		if hosts := r.findReverse(name); len(hosts) > 0 {
			return dnsPtrRecord(query, name, hosts), nil
		}
	}

	return dnsNotFound(query), nil
}

func (r *dnsResolver) findHost(name string) (addrs []net.IP) {
	r.hostMutex.RLock()
	defer r.hostMutex.RUnlock()

	for _, hosts := range r.hosts {
		for _, hostName := range hosts.Names {
			if dns.Fqdn(hostName) == name {
				addrs = append(addrs, hosts.Address)
			}
		}
	}
	return
}

func (r *dnsResolver) findReverse(address string) (hosts []string) {
	r.hostMutex.RLock()
	defer r.hostMutex.RUnlock()

	address = strings.ToLower(dns.Fqdn(address))

	for _, entry := range r.hosts {
		if r, _ := dns.ReverseAddr(entry.Address.String()); address == r && len(entry.Names) > 0 {
			hosts = append(hosts, dns.Fqdn(entry.Names[0]))
		}
	}
	return
}

func dnsAddressRecord(query *dns.Msg, name string, addrs []net.IP) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetReply(query)
	for _, addr := range addrs {
		rr := new(dns.A)
		rr.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 0}
		rr.A = addr

		resp.Answer = append(resp.Answer, rr)
	}
	return resp
}

func dnsPtrRecord(query *dns.Msg, name string, hosts []string) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetReply(query)
	for _, host := range hosts {
		rr := new(dns.PTR)
		rr.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: 0}
		rr.Ptr = host

		resp.Answer = append(resp.Answer, rr)
	}
	return resp
}

func dnsNotFound(query *dns.Msg) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetReply(query)
	resp.SetRcode(query, dns.RcodeNameError)
	return resp
}
