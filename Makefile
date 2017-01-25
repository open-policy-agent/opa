# Copyright 2016 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

VERSION := 0.4.0

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
GOARCH := $(shell go env GOARCH)
GOOS := $(shell go env GOOS)
DISABLE_CGO := CGO_ENABLED=0

# RELEASE_BUILDER_GO_VERSION defines the version of Go used to build the
# release. This should be kept in sync with the Go versions in .travis.yml.
RELEASE_BUILDER_GO_VERSION := 1.7

BIN := opa_$(GOOS)_$(GOARCH)

REPOSITORY := openpolicyagent
IMAGE := $(REPOSITORY)/opa

BUILD_COMMIT := $(shell ./build/get-build-commit.sh)
BUILD_TIMESTAMP := $(shell ./build/get-build-timestamp.sh)
BUILD_HOSTNAME := $(shell ./build/get-build-hostname.sh)

LDFLAGS := "-X github.com/open-policy-agent/opa/version.Version=$(VERSION) \
	-X github.com/open-policy-agent/opa/version.Vcs=$(BUILD_COMMIT) \
	-X github.com/open-policy-agent/opa/version.Timestamp=$(BUILD_TIMESTAMP) \
	-X github.com/open-policy-agent/opa/version.Hostname=$(BUILD_HOSTNAME)"

GO15VENDOREXPERIMENT := 1
export GO15VENDOREXPERIMENT

.PHONY: all deps generate build install test perf perf-regression cover check check-fmt check-vet check-lint fmt clean \
	release-builder push-release-builder release release-patch

######################################################
#
# Development targets
#
######################################################

all: deps build test check

deps:
	@./build/install-deps.sh

generate:
	$(GO) generate

build: generate
	$(DISABLE_CGO) $(GO) build -o $(BIN) -ldflags $(LDFLAGS)

image:
	@$(MAKE) build GOOS=linux
	@$(MAKE) image-quick

image-quick:
	sed -e 's/GOARCH/$(GOARCH)/g' Dockerfile.in > .Dockerfile_$(GOARCH)
	docker build -t $(IMAGE):$(VERSION)	-f .Dockerfile_$(GOARCH) .

push:
	docker push $(IMAGE):$(VERSION)

tag-latest:
	docker tag $(IMAGE):$(VERSION) $(IMAGE):latest

push-latest:
	docker push $(IMAGE):latest

install: generate
	$(DISABLE_CGO) $(GO) install -ldflags $(LDFLAGS)

test: generate
	$(DISABLE_CGO) $(GO) test $(PACKAGES)

COVER_PACKAGES=$(PACKAGES)
$(COVER_PACKAGES):
	@mkdir -p coverage/$(shell dirname $@)
	$(DISABLE_CGO) $(GO) test -covermode=count -coverprofile=coverage/$(shell dirname $@)/coverage.out $@
	$(GO) tool cover -html=coverage/$(shell dirname $@)/coverage.out || true

perf: generate
	$(DISABLE_CGO) $(GO) test -v -run=donotruntests -bench=. $(PACKAGES) | grep "^Benchmark"

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
	rm -f opa_*_*
	rm -f .Dockerfile_*

######################################################
#
# Release targets
#
######################################################

release-builder:
	sed -e 's/GOVERSION/$(RELEASE_BUILDER_GO_VERSION)/g' Dockerfile_release-builder.in > .Dockerfile_release-builder
	docker build -f .Dockerfile_release-builder -t $(REPOSITORY)/release-builder .

push-release-builder:
	docker push $(REPOSITORY)/release-builder

release:
	docker run -it --rm -v $(PWD)/_release/$(VERSION):/_release/$(VERSION) \
		-v $(PWD)/build/build-release.sh:/build-release.sh \
		$(REPOSITORY)/release-builder:latest \
		/build-release.sh $(VERSION) /_release/$(VERSION)

release-patch:
	@docker run -it --rm -v $(PWD)/build/gen-release-patch.sh:/gen-release-patch.sh \
		python:2.7 /gen-release-patch.sh $(VERSION)
