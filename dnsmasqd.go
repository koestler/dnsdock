package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

func ipAddress() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && !ipnet.IP.IsMulticast() {
			if ipv4 := ipnet.IP.To4(); ipv4 != nil {
				return ipv4.String(), nil
			}
		}
	}

	return "", errors.New("no addresses found")
}

func writeDnsmasqd() error {
	address, err := ipAddress()
	if err != nil {
		return err
	}
	log.Println("got local address:", address)

	filepath := "/etc/dnsmasq.d/dnsdock"

	log.Printf("write dnsmasq configurtion to: %s", filepath)

	// generate configuration file
	conf := make([]string, 0, 17)

	// add forward dns for *.docker
	conf = append(conf, fmt.Sprintf("server=/docker/%s", address))

	// add reverse dns for 172.16.0.0/12
	for i := 16; i < 32; i++ {
		conf = append(conf, fmt.Sprintf("server=/%d.172.in-addr.arpa/%s", i, address))
	}

	// write configuration file
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(strings.Join(conf, "\n") + "\n")
	return err
}
