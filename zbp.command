#!/bin/bash
cd $(dirname $BASH_SOURCE) || {
    echo Error getting script directory >&2
    exit 1
}
go version
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GO111MODULE=auto
while true
do
    go mod tidy
    #go build -ldflags="-s -w" -o ZeroBot-Plugin
    go generate main.go
    go run -ldflags "-s -checklinkname=0" main.go -c config.json
done
