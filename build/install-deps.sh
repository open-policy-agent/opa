#!/usr/bin/env bash

set -e

which pigeon >/dev/null || {
	echo "Installing pigeon from vendor"
	go install ./vendor/github.com/mna/pigeon
}


which goimports >/dev/null || {
	echo "Installing goimports from vendor"
	go install ./vendor/golang.org/x/tools/cmd/goimports
}

which golint >/dev/null || {
	echo "Installing golint from vendor"
	go install ./vendor/github.com/golang/lint/golint
}
