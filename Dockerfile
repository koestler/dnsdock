FROM golang:1.11-stretch as gobuild

COPY . /go/src/github.com/koestler/dnsdock/
RUN cd /go/src/github.com/koestler/dnsdock/ && ./build.sh

FROM scratch
COPY --from=gobuild /go/src/github.com/koestler/dnsdock/dnsdock /dnsdock
ENTRYPOINT ["/dnsdock"]
