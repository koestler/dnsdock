package resolver

import (
	"fmt"
	"github.com/koestler/dnsdock/dnsStorage"
	"github.com/miekg/dns"
	"log"
	"net"
)

type Resolver interface {
	RemoveHost(id string) error

	Listen() error
	Close()
}

type DnsResolver struct {
	Storage *dnsStorage.DnsStorage

	Port       int
	serverUdp  *dns.Server
	serverTcp  *dns.Server
	stoppedUdp chan struct{}
	stoppedTcp chan struct{}
}

func NewResolver(storage *dnsStorage.DnsStorage) (*DnsResolver, error) {
	return &DnsResolver{
		Storage:    storage,
		Port:       53,
		stoppedUdp: make(chan struct{}),
		stoppedTcp: make(chan struct{}),
	}, nil
}

func (r *DnsResolver) RemoveHost(id string) error {
	r.Storage.RemoveHost(id)
	return nil
}

func (r *DnsResolver) Listen() error {
	addr := fmt.Sprintf(":%d", r.Port)

	// create UDP listener
	listenAddrUdp, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return err
	}

	connUdp, err := net.ListenUDP("udp4", listenAddrUdp)
	if err != nil {
		return err
	}

	// create TCP listener
	listenAddrTcp, err := net.ResolveTCPAddr("tcp4", addr)
	if err != nil {
		return err
	}

	connTcp, err := net.ListenTCP("tcp4", listenAddrTcp)
	if err != nil {
		return err
	}

	// start DNS server
	startupErrorUdp := make(chan error)
	startupErrorTcp := make(chan error)

	r.serverUdp = &dns.Server{Handler: r, PacketConn: connUdp, NotifyStartedFunc: func() {
		startupErrorUdp <- nil
	}}
	r.serverTcp = &dns.Server{Handler: r, Listener: connTcp, NotifyStartedFunc: func() {
		startupErrorTcp <- nil
	}}

	go func() {
		select {
		case startupErrorUdp <- r.runUdp():
		default:
		}
	}()

	go func() {
		select {
		case startupErrorTcp <- r.runTcp():
		default:
		}
	}()

	errorUdp := <-startupErrorUdp
	errorTcp := <-startupErrorTcp

	if errorUdp != nil {
		return errorUdp
	}
	return errorTcp
}

func (r *DnsResolver) runUdp() error {
	defer close(r.stoppedUdp)
	return r.serverUdp.ActivateAndServe()
}

func (r *DnsResolver) runTcp() error {
	defer close(r.stoppedTcp)
	return r.serverTcp.ActivateAndServe()
}

func (r *DnsResolver) Wait() error {
	<-r.stoppedUdp
	<-r.stoppedTcp
	return nil
}

func (r *DnsResolver) Close() {
	if r.serverUdp != nil {
		r.serverUdp.Shutdown()
	}
	if r.serverTcp != nil {
		r.serverTcp.Shutdown()
	}
}

func (r *DnsResolver) ServeDNS(w dns.ResponseWriter, query *dns.Msg) {
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

func (r *DnsResolver) responseForQuery(query *dns.Msg) (*dns.Msg, error) {
	// answer to first question
	name := query.Question[0].Name

	if query.Question[0].Qtype == dns.TypeA {
		if addrs := r.Storage.FindHostAddresses(name); len(addrs) > 0 {
			return dnsAddressRecord(query, name, addrs), nil
		}
	} else if query.Question[0].Qtype == dns.TypePTR {
		if hosts := r.Storage.FindReverseHost(name); len(hosts) > 0 {
			return dnsPtrRecord(query, name, hosts), nil
		}
	}

	return dnsNotFound(query), nil
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
