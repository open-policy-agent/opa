#!/bin/sh

GOOS="" GOARCH="" go run ./vendor/golang.org/x/tools/cmd/goimports $@
