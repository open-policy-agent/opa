#!/bin/sh

GOOS="" GOARCH="" go run -tags=opa_wasm ./vendor/golang.org/x/tools/cmd/goimports -local github.com/open-policy-agent/opa $@
