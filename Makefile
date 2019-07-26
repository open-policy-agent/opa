# Copyright 2016 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

VERSION := 0.13.0-dev

GO := go
GOVERSION := 1.11
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

RELEASE_BUILDER_VERSION := 1.2

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

.PHONY: all
all: deps build test perf check

.PHONY: version
version:
	@echo $(VERSION)

.PHONY: deps
deps:
	@./build/install-deps.sh

.PHONY: wasm-build
wasm-build:
ifeq ($(DOCKER_INSTALLED), 1)
	@$(MAKE) -C wasm build
	cp wasm/_obj/opa.wasm internal/compiler/wasm/opa/opa.wasm
else
	@echo "Docker not installed. Skipping OPA-WASM library build."
endif

.PHONY: generate
generate: wasm-build
	$(GO) generate

.PHONY: build
build: go-build

.PHONY: go-build
go-build: generate
	$(GO) build -o $(BIN) -ldflags $(LDFLAGS)

.PHONY: image
image:
	@$(MAKE) build GOOS=linux
	@$(MAKE) image-quick

.PHONY: image-quick
image-quick:
	sed -e 's/GOARCH/$(GOARCH)/g' Dockerfile.in > .Dockerfile_$(GOARCH)
	sed -e 's/GOARCH/$(GOARCH)/g' Dockerfile_debug.in > .Dockerfile_debug_$(GOARCH)
	sed -e 's/GOARCH/$(GOARCH)/g' Dockerfile_rootless.in > .Dockerfile_rootless_$(GOARCH)
	docker build -t $(IMAGE):$(VERSION)	-f .Dockerfile_$(GOARCH) .
	docker build -t $(IMAGE):$(VERSION)-debug -f .Dockerfile_debug_$(GOARCH) .
	docker build -t $(IMAGE):$(VERSION)-rootless -f .Dockerfile_rootless_$(GOARCH) .

.PHONY: push
push:
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):$(VERSION)-debug
	docker push $(IMAGE):$(VERSION)-rootless

.PHONY: tag-latest
tag-latest:
	docker tag $(IMAGE):$(VERSION) $(IMAGE):latest
	docker tag $(IMAGE):$(VERSION)-debug $(IMAGE):latest-debug
	docker tag $(IMAGE):$(VERSION)-rootless $(IMAGE):latest-rootless

.PHONY: push-latest
push-latest:
	docker push $(IMAGE):latest
	docker push $(IMAGE):latest-debug
	docker push $(IMAGE):latest-rootless

.PHONY: push-binary-edge
push-binary-edge:
	aws s3 cp opa_linux_amd64 s3://opa-releases/edge/opa_linux_amd64

.PHONY: docker-login
docker-login:
	@docker login -u ${DOCKER_USER} -p ${DOCKER_PASSWORD}

.PHONY: deploy-travis
deploy-travis: docker-login image-quick push push-binary-edge

.PHONY: release-travis
release-travis: deploy-travis tag-latest push-latest

.PHONY: release-bugfix-travis
release-bugfix-travis: deploy-travis

.PHONY: install
install: generate
	$(GO) install -ldflags $(LDFLAGS)

.PHONY: test
test: opa-wasm-test go-test wasm-test

.PHONY: opa-wasm-test
opa-wasm-test:
ifeq ($(DOCKER_INSTALLED), 1)
	@$(MAKE)  -C wasm test
else
	@echo "Docker not installed. Skipping OPA-WASM library test."
endif

.PHONY: go-test
go-test: generate
	$(GO) test ./...

.PHONY: wasm-test
wasm-test: generate
ifeq ($(DOCKER_INSTALLED), 1)
	@./build/run-wasm-tests.sh
else
	@echo "Docker not installed. Skipping WASM-based test execution."
endif

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
	$(GO) fmt ./...

.PHONY: wasm-clean
wasm-clean:
	@$(MAKE) -C wasm clean

.PHONY: clean
clean: wasm-clean
	rm -f .Dockerfile_*
	rm -f opa_*_*
	rm -fr _test

# The docs-% pattern target will shim to the
# makefile in ./docs
.PHONY: docs-%
docs-%:
	$(MAKE) -C docs $*

######################################################
#
# Release targets
#
######################################################

.PHONY: release-builder
release-builder:
	sed -e s/GOVERSION/$(GOVERSION)/g Dockerfile_release-builder.in > .Dockerfile_release-builder
	docker build -f .Dockerfile_release-builder -t $(REPOSITORY)/release-builder:$(RELEASE_BUILDER_VERSION) -t $(REPOSITORY)/release-builder:latest .

.PHONY: push-release-builder
push-release-builder:
	docker push $(REPOSITORY)/release-builder:latest
	docker push $(REPOSITORY)/release-builder:$(RELEASE_BUILDER_VERSION)

.PHONY: release
release:
	docker run -it --rm \
		-v $(PWD)/_release/$(VERSION):/_release/$(VERSION) \
		-v $(PWD):/_src \
		$(REPOSITORY)/release-builder:$(RELEASE_BUILDER_VERSION) \
		/_src/build/build-release.sh --version=$(VERSION) --output-dir=/_release/$(VERSION) --source-url=/_src

.PHONY: release-local
release-local:
	docker run -it --rm \
		-v $(PWD)/_release/$(VERSION):/_release/$(VERSION) \
		-v $(PWD):/_src \
		$(REPOSITORY)/release-builder:$(RELEASE_BUILDER_VERSION) \
		/_src/build/build-release.sh --output-dir=/_release/$(VERSION) --source-url=/_src

.PHONY: release-patch
release-patch:
	@docker run -it --rm \
		-e LAST_VERSION=$(LAST_VERSION) \
		-v $(PWD):/_src \
		python:2.7 \
		/_src/build/gen-release-patch.sh --version=$(VERSION) --source-url=/_src

.PHONY: dev-patch
dev-patch:
	@docker run -it --rm \
		-v $(PWD):/_src \
		python:2.7 \
		/_src/build/gen-dev-patch.sh --version=$(VERSION) --source-url=/_src
