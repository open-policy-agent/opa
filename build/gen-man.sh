#!/bin/sh

GOOS="" GOARCH="" go run ./build/generate-man $@
