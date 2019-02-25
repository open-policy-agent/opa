# Copyright 2016 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

VERSION := 0.10.5

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

RELEASE_BUILDER_VERSION := 1.1

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
	docker build -t $(IMAGE):$(VERSION)	-f .Dockerfile_$(GOARCH) .
	docker build -t $(IMAGE):$(VERSION)-debug -f .Dockerfile_debug_$(GOARCH) .

.PHONY: push
push:
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):$(VERSION)-debug

.PHONY: tag-latest
tag-latest:
	docker tag $(IMAGE):$(VERSION) $(IMAGE):latest
	docker tag $(IMAGE):$(VERSION)-debug $(IMAGE):latest-debug

.PHONY: push-latest
push-latest:
	docker push $(IMAGE):latest
	docker push $(IMAGE):latest-debug

.PHONY: docker-login
docker-login:
	@docker login -u ${DOCKER_USER} -p ${DOCKER_PASSWORD}

.PHONY: deploy-travis
deploy-travis: docker-login image-quick push

.PHONY: release-travis
release-travis: deploy-travis tag-latest push-latest

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
	@mkdir -p _test
	@$(GO) run test/wasm/cmd/testgen.go --input-dir test/wasm/assets --output _test/testcases.tar.gz
	@$(DOCKER) run -it --rm -e VERBOSE=$(VERBOSE) -v $(PWD):/src -w /src node:8 ./build/test-wasm.sh
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
	rm -fr site.tar.gz docs/_site docs/node_modules docs/book/_book
	rm -fr _test

.PHONY: docs
docs:
	docker run -it --rm \
		-v $(PWD):/go/src/github.com/open-policy-agent/opa \
		-w /go/src/github.com/open-policy-agent/opa \
		-p 4000:4000 \
		-u $(shell id -u $(USER)):$(shell id -g $(USER)) \
		-e GITBOOK_DIR=/go/src/github.com/open-policy-agent/opa/.gitbook \
		-e npm_config_cache=/go/src/github.com/open-policy-agent/opa/.npm \
		$(REPOSITORY)/release-builder:$(RELEASE_BUILDER_VERSION) \
		./build/build-docs.sh --output-dir=/go/src/github.com/open-policy-agent/opa --serve=4000


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
