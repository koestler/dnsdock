#! /usr/bin/env bash

set +x

go get
go build

mv resolvable /bin/resolvable
