docker run --rm -it \
       -v `pwd`:/go/src/github.com/koestler/dnsdock \
       -v /var/run/docker.sock:/var/run/docker.sock \
       -v /etc/NetworkManager/dnsmasq.d/:/etc/dnsmasq.d/ \
       golang:1.8-wheezy bash
# now run cd src/github.com/koestler/dnsdock && ./build.sh
