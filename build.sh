#! /usr/bin/env bash

set +x

go get
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -a -tags netgo -ldflags '-w'

mv dnsdock /go/bin/dnsdock
