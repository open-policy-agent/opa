# Copyright 2016 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

PACKAGES := \
	github.com/open-policy-agent/opa/ast/.../ \
	github.com/open-policy-agent/opa/cmd/.../ \
	github.com/open-policy-agent/opa/eval/.../ \
	github.com/open-policy-agent/opa/runtime/.../ \
	github.com/open-policy-agent/opa/util/.../

BUILD_COMMIT := $(shell ./build/get-build-commit.sh)
BUILD_TIMESTAMP := $(shell ./build/get-build-timestamp.sh)
BUILD_HOSTNAME := $(shell ./build/get-build-hostname.sh)

LDFLAGS := -ldflags "-X github.com/open-policy-agent/opa/version.Vcs=$(BUILD_COMMIT) \
	-X github.com/open-policy-agent/opa/version.Timestamp=$(BUILD_TIMESTAMP) \
	-X github.com/open-policy-agent/opa/version.Hostname=$(BUILD_HOSTNAME)"

GO := go

GO15VENDOREXPERIMENT := 1
export GO15VENDOREXPERIMENT

.PHONY: all deps generate build test cover check check-fmt check-vet check-lint fmt clean

all: build test check

deps:
	$(GO) install ./vendor/github.com/PuerkitoBio/pigeon
	$(GO) install ./vendor/golang.org/x/tools/cmd/goimports
	$(GO) install ./vendor/github.com/golang/lint/golint

generate:
	$(GO) generate

build: generate
	$(GO) build -o opa $(LDFLAGS)

test: generate
	$(GO) test -v $(PACKAGES)

COVER_PACKAGES=$(PACKAGES)
$(COVER_PACKAGES):
	@mkdir -p coverage/$(shell dirname $@)
	go test -covermode=count -coverprofile=coverage/$(shell dirname $@)/coverage.out $@
	go tool cover -html=coverage/$(shell dirname $@)/coverage.out || true

cover: $(COVER_PACKAGES)

check: check-fmt check-vet check-lint

check-fmt:
	./build/check-fmt.sh

check-vet:
	./build/check-vet.sh

check-lint:
	./build/check-lint.sh

fmt:
	$(GO) fmt $(PACKAGES)

clean:
	rm -f ./opa
