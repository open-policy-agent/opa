#!/bin/sh

GOFLAGS=-mod=vendor GO111MODULE=on GOOS="" GOARCH="" go run ./vendor/github.com/mna/pigeon $@
