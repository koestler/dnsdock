VERSION=`git symbolic-ref -q --short HEAD || git describe --tags --exact-match`
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -a -tags netgo -ldflags="-s -w -X main.buildVersion=$VERSION -X main.buildTime=`date -Is`"
