#!/bin/sh

GOFLAGS=-mod=vendor GO111MODULE=on GOOS="" GOARCH="" go run -tags=wasmer ./vendor/golang.org/x/tools/cmd/goimports -local github.com/open-policy-agent/opa $@
