# Copyright 2016 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

PACKAGES := \
	github.com/open-policy-agent/opa/ast/.../ \
	github.com/open-policy-agent/opa/cmd/.../ \
	github.com/open-policy-agent/opa/repl/.../ \
	github.com/open-policy-agent/opa/runtime/.../ \
	github.com/open-policy-agent/opa/storage/.../ \
	github.com/open-policy-agent/opa/topdown/.../ \
	github.com/open-policy-agent/opa/util/.../ \
	github.com/open-policy-agent/opa/test/.../

GO := go
GOX := gox

BUILD_COMMIT := $(shell ./build/get-build-commit.sh)
BUILD_TIMESTAMP := $(shell ./build/get-build-timestamp.sh)
BUILD_HOSTNAME := $(shell ./build/get-build-hostname.sh)

LDFLAGS := -ldflags "-X github.com/open-policy-agent/opa/version.Vcs=$(BUILD_COMMIT) \
	-X github.com/open-policy-agent/opa/version.Timestamp=$(BUILD_TIMESTAMP) \
	-X github.com/open-policy-agent/opa/version.Hostname=$(BUILD_HOSTNAME)"

# Set CROSSCOMPILE to space separated list of <platform>/<arch> pairs
# and "gox" will be used to build the binaries instead of "go".
CROSSCOMPILE ?=

GO15VENDOREXPERIMENT := 1
export GO15VENDOREXPERIMENT

.PHONY: all deps generate build test cover check check-fmt check-vet check-lint fmt clean

all: build test check

deps:
	$(GO) install ./vendor/github.com/PuerkitoBio/pigeon
	$(GO) install ./vendor/golang.org/x/tools/cmd/goimports
	$(GO) install ./vendor/github.com/golang/lint/golint
	$(GO) get github.com/mitchellh/gox

generate:
	$(GO) generate

build: generate
ifeq ($(CROSSCOMPILE),)
	$(GO) build -o opa $(LDFLAGS)
else
	$(GOX) -osarch="$(CROSSCOMPILE)" $(LDFLAGS)
endif

install: generate
	$(GO) install $(LDFLAGS)

test: generate
	$(GO) test -v $(PACKAGES)

COVER_PACKAGES=$(PACKAGES)
$(COVER_PACKAGES):
	@mkdir -p coverage/$(shell dirname $@)
	$(GO) test -covermode=count -coverprofile=coverage/$(shell dirname $@)/coverage.out $@
	$(GO) tool cover -html=coverage/$(shell dirname $@)/coverage.out || true

perf: generate
	$(GO) test -v -run=donotruntests -bench=. $(PACKAGES) | grep "^Benchmark"

perf-regression:
	./build/run-perf-regression.sh

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
	rm -f ./opa_linux_amd64
	rm -f ./opa_darwin_amd64
