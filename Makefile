# Copyright 2016 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

VERSION := 0.1.1-dev

PACKAGES := \
	github.com/open-policy-agent/opa/ast/.../ \
	github.com/open-policy-agent/opa/cmd/.../ \
	github.com/open-policy-agent/opa/repl/.../ \
	github.com/open-policy-agent/opa/runtime/.../ \
	github.com/open-policy-agent/opa/server/.../ \
	github.com/open-policy-agent/opa/storage/.../ \
	github.com/open-policy-agent/opa/topdown/.../ \
	github.com/open-policy-agent/opa/util/.../ \
	github.com/open-policy-agent/opa/test/.../

GO := go

BIN := opa

REPOSITORY := openpolicyagent
IMAGE := $(REPOSITORY)/opa

BUILD_COMMIT := $(shell ./build/get-build-commit.sh)
BUILD_TIMESTAMP := $(shell ./build/get-build-timestamp.sh)
BUILD_HOSTNAME := $(shell ./build/get-build-hostname.sh)

LDFLAGS := "-X github.com/open-policy-agent/opa/version.Version=$(VERSION) \
	-X github.com/open-policy-agent/opa/version.Vcs=$(BUILD_COMMIT) \
	-X github.com/open-policy-agent/opa/version.Timestamp=$(BUILD_TIMESTAMP) \
	-X github.com/open-policy-agent/opa/version.Hostname=$(BUILD_HOSTNAME) \
	-s"

GO15VENDOREXPERIMENT := 1
export GO15VENDOREXPERIMENT

.PHONY: all deps generate build install test perf perf-regression cover check check-fmt check-vet check-lint fmt clean

all: deps build test check

deps:
	@./build/install-deps.sh

generate:
	$(GO) generate

build: generate
	CGO_ENABLED=0 $(GO) build -o $(BIN) -ldflags $(LDFLAGS)

image: build
	docker build -t $(IMAGE):$(VERSION) .

push:
	docker push $(IMAGE):$(VERSION)

install: generate
	$(GO) install -ldflags $(LDFLAGS)

test: generate
	$(GO) test $(PACKAGES)

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
	rm -f opa
