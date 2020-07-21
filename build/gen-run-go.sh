#!/bin/sh

set -e

GOFLAGS=-mod=vendor GO111MODULE=on GOOS="" GOARCH="" go run $@