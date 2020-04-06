#!/bin/sh

GOFLAGS=-mod=vendor GO111MODULE=on GOOS="" GOARCH="" go run ./build/generate-man $@
