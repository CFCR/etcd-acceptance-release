#!/bin/bash

export GOPATH=/var/vcap/packages/acceptance
export GOROOT=/var/vcap/packages/golang
export PATH="$PATH:$GOPATH/bin:$GOROOT/bin"

cd /var/vcap/packages/acceptance/src/acceptance
go install ./vendor/github.com/onsi/ginkgo/ginkgo
