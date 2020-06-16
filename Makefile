# Copyright 2016 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

VERSION := 0.21.0

CGO_ENABLED ?= 0

# Force modules on and to use the vendor directory.
GO := CGO_ENABLED=$(CGO_ENABLED) GO111MODULE=on GOFLAGS=-mod=vendor go

GOVERSION := $(shell cat ./.go-version)
GOARCH := $(shell go env GOARCH)
GOOS := $(shell go env GOOS)

DOCKER_INSTALLED := $(shell hash docker 2>/dev/null && echo 1 || echo 0)
DOCKER := docker

BIN := opa_$(GOOS)_$(GOARCH)

REPOSITORY := openpolicyagent
IMAGE := $(REPOSITORY)/opa

BUILD_COMMIT := $(shell ./build/get-build-commit.sh)
BUILD_TIMESTAMP := $(shell ./build/get-build-timestamp.sh)
BUILD_HOSTNAME := $(shell ./build/get-build-hostname.sh)

RELEASE_BUILD_IMAGE := golang:$(GOVERSION)

LDFLAGS := "-X github.com/open-policy-agent/opa/version.Version=$(VERSION) \
	-X github.com/open-policy-agent/opa/version.Vcs=$(BUILD_COMMIT) \
	-X github.com/open-policy-agent/opa/version.Timestamp=$(BUILD_TIMESTAMP) \
	-X github.com/open-policy-agent/opa/version.Hostname=$(BUILD_HOSTNAME)"

GO15VENDOREXPERIMENT := 1
export GO15VENDOREXPERIMENT

######################################################
#
# Development targets
#
######################################################

# If you update the 'all' target make sure the 'travis' target is consistent.
.PHONY: all
all: build test perf check

.PHONY: version
version:
	@echo $(VERSION)

.PHONY: generate
generate: wasm-lib-build
	$(GO) generate

.PHONY: build
build: go-build

.PHONY: image
image: build-linux
	@$(MAKE) image-quick

.PHONY: install
install: generate
	$(GO) install -ldflags $(LDFLAGS)

.PHONY: test
test: go-test wasm-test

.PHONY: go-build
go-build: generate
	$(GO) build -o $(BIN) -ldflags $(LDFLAGS)

.PHONY: go-test
go-test: generate
	$(GO) test -tags=slow ./...

.PHONY: perf
perf: generate
	$(GO) test -run=- -bench=. -benchmem ./...

.PHONY: check
check: check-fmt check-vet check-lint

.PHONY: check-fmt
check-fmt:
	./build/check-fmt.sh

.PHONY: check-vet
check-vet:
	./build/check-vet.sh

.PHONY: check-lint
check-lint:
	./build/check-lint.sh

.PHONY: fmt
fmt:
	./build/run-fmt.sh

.PHONY: clean
clean: wasm-lib-clean
	rm -f opa_*_*

######################################################
#
# Documentation targets
#
######################################################

# The docs-% pattern target will shim to the
# makefile in ./docs
.PHONY: docs-%
docs-%:
	$(MAKE) -C docs $*

.PHONY: man
man:
	./build/gen-man.sh man

######################################################
#
# Linux distro package targets
#
######################################################

.PHONY: deb
deb:
	VERSION=$(VERSION) ./build/gen-deb.sh

######################################################
#
# Wasm targets
#
######################################################

.PHONY: wasm-test
wasm-test: wasm-lib-test wasm-rego-test

.PHONY: wasm-lib-build
wasm-lib-build:
ifeq ($(DOCKER_INSTALLED), 1)
	@$(MAKE) -C wasm build
	cp wasm/_obj/opa.wasm internal/compiler/wasm/opa/opa.wasm
else
	@echo "Docker not installed. Skipping OPA-WASM library build."
endif

.PHONY: wasm-lib-test
wasm-lib-test:
ifeq ($(DOCKER_INSTALLED), 1)
	@$(MAKE) -C wasm test
else
	@echo "Docker not installed. Skipping OPA-WASM library test."
endif

.PHONY: wasm-rego-test
wasm-rego-test: generate
ifeq ($(DOCKER_INSTALLED), 1)
	GOVERSION=$(GOVERSION) ./build/run-wasm-rego-tests.sh
else
	@echo "Docker not installed. Skipping Rego-WASM test."
endif

.PHONY: wasm-lib-clean
wasm-lib-clean:
	@$(MAKE) -C wasm clean

.PHONY: wasm-rego-testgen-install
wasm-rego-testgen-install:
	$(GO) install ./test/wasm/cmd/wasm-rego-testgen

######################################################
#
# CI targets
#
######################################################

.PHONY: travis-go
travis-go:
	$(DOCKER) run \
		--rm \
		-u $(shell id -u):$(shell id -g) \
		-v $(PWD):/src \
		-w /src \
		-e GOCACHE=/src/.go/cache \
		golang:$(GOVERSION) \
		make build-linux build-windows build-darwin go-test perf travis-check

.PHONY: travis-check
travis-check: check
	./build/check-working-copy.sh

# The travis-wasm target exists because we do not want to run the generate
# target outside of Docker. This step duplicates the the wasm-rego-test target
# above.
.PHONY: travis-wasm
travis-wasm: wasm-lib-test
	GOVERSION=$(GOVERSION) ./build/run-wasm-rego-tests.sh

.PHONY: travis
travis: travis-go travis-wasm

.PHONY: build-linux
build-linux:
	@$(MAKE) build GOOS=linux

.PHONY: build-darwin
build-darwin:
	@$(MAKE) build GOOS=darwin

.PHONY: build-windows
build-windows:
	@$(MAKE) build GOOS=windows
	mv opa_windows_$(GOARCH) opa_windows_$(GOARCH).exe

.PHONY: image-quick
image-quick:
	$(DOCKER) build -t $(IMAGE):$(VERSION) --build-arg BASE=scratch .
	$(DOCKER) build -t $(IMAGE):$(VERSION)-debug --build-arg BASE=gcr.io/distroless/base:debug .
	$(DOCKER) build -t $(IMAGE):$(VERSION)-rootless --build-arg USER=1000 --build-arg BASE=scratch .

.PHONY: push
push:
	$(DOCKER) push $(IMAGE):$(VERSION)
	$(DOCKER) push $(IMAGE):$(VERSION)-debug
	$(DOCKER) push $(IMAGE):$(VERSION)-rootless

.PHONY: tag-latest
tag-latest:
	$(DOCKER) tag $(IMAGE):$(VERSION) $(IMAGE):latest
	$(DOCKER) tag $(IMAGE):$(VERSION)-debug $(IMAGE):latest-debug
	$(DOCKER) tag $(IMAGE):$(VERSION)-rootless $(IMAGE):latest-rootless

.PHONY: push-latest
push-latest:
	$(DOCKER) push $(IMAGE):latest
	$(DOCKER) push $(IMAGE):latest-debug
	$(DOCKER) push $(IMAGE):latest-rootless

.PHONY: push-binary-edge
push-binary-edge:
	aws s3 cp opa_darwin_$(GOARCH) s3://opa-releases/edge/opa_darwin_$(GOARCH)
	aws s3 cp opa_windows_$(GOARCH).exe s3://opa-releases/edge/opa_windows_$(GOARCH).exe
	aws s3 cp opa_linux_$(GOARCH) s3://opa-releases/edge/opa_linux_$(GOARCH)

.PHONY: tag-edge
tag-edge:
	$(DOCKER) tag $(IMAGE):$(VERSION) $(IMAGE):edge
	$(DOCKER) tag $(IMAGE):$(VERSION)-debug $(IMAGE):edge-debug
	$(DOCKER) tag $(IMAGE):$(VERSION)-rootless $(IMAGE):edge-rootless

.PHONY: push-edge
push-edge:
	$(DOCKER) push $(IMAGE):edge
	$(DOCKER) push $(IMAGE):edge-debug
	$(DOCKER) push $(IMAGE):edge-rootless

.PHONY: docker-login
docker-login:
	@$(DOCKER) login -u ${DOCKER_USER} -p ${DOCKER_PASSWORD}

.PHONY: push-image
push-image: docker-login image-quick push

.PHONY: deploy-travis
deploy-travis: push-image tag-edge push-edge push-binary-edge

.PHONY: release-travis
# Don't tag and push "latest" image tags if the version is a release candidate
ifneq (,$(findstring rc,$(VERSION)))
release-travis: push-image
else
release-travis: push-image tag-latest push-latest
endif

.PHONY: release-bugfix-travis
release-bugfix-travis: deploy-travis

.PHONY: netlify-prod
netlify-prod: clean docs-clean build docs-generate docs-production-build

.PHONY: netlify-preview
netlify-preview: clean docs-clean build docs-live-blocks-install-deps docs-live-blocks-test docs-generate docs-preview-build

######################################################
#
# Release targets
#
######################################################

.PHONY: release
release:
	$(DOCKER) run -it --rm \
		-v $(PWD)/_release/$(VERSION):/_release/$(VERSION) \
		-v $(PWD):/_src \
		$(RELEASE_BUILD_IMAGE) \
		/_src/build/build-release.sh --version=$(VERSION) --output-dir=/_release/$(VERSION) --source-url=/_src

.PHONY: release-local
release-local:
	$(DOCKER) run -it --rm \
		-v $(PWD)/_release/$(VERSION):/_release/$(VERSION) \
		-v $(PWD):/_src \
		$(RELEASE_BUILD_IMAGE) \
		/_src/build/build-release.sh --output-dir=/_release/$(VERSION) --source-url=/_src

.PHONY: release-patch
release-patch:
	@$(DOCKER) run -it --rm \
		-e LAST_VERSION=$(LAST_VERSION) \
		-v $(PWD):/_src \
		python:2.7 \
		/_src/build/gen-release-patch.sh --version=$(VERSION) --source-url=/_src

.PHONY: dev-patch
dev-patch:
	@$(DOCKER) run -it --rm \
		-v $(PWD):/_src \
		python:2.7 \
		/_src/build/gen-dev-patch.sh --version=$(VERSION) --source-url=/_src
