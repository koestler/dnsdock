FROM golang:1.8-wheezy

COPY . /go/src/github.com/koestler/resolvable/
RUN cd /go/src/github.com/koestler/resolvable/ && ./build.sh

ENTRYPOINT ["/bin/resolvable"]
