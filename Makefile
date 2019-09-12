# Copyright 2016 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

VERSION := 0.15.0-dev

GO := go
GOVERSION := 1.12.9
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

# If you update the 'all' target check/update the call in Dockerfile.build target
# to make sure they're consistent.
.PHONY: all
all: deps build test perf check

.PHONY: version
version:
	@echo $(VERSION)

.PHONY: deps
deps:
	@./build/install-deps.sh

.PHONY: generate
generate: wasm-build
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
test: wasm-test go-test wasm-rego-test

.PHONY: go-build
go-build: generate
	$(GO) build -o $(BIN) -ldflags $(LDFLAGS)

.PHONY: go-test
go-test: generate
	$(GO) test ./...

.PHONY: wasm-build
wasm-build:
ifeq ($(DOCKER_INSTALLED), 1)
	@$(MAKE) -C wasm build
	cp wasm/_obj/opa.wasm internal/compiler/wasm/opa/opa.wasm
else
	@echo "Docker not installed. Skipping OPA-WASM library build."
endif

.PHONY: wasm-clean
wasm-clean:
	@$(MAKE) -C wasm clean

.PHONY: wasm-test
wasm-test:
ifeq ($(DOCKER_INSTALLED), 1)
	@$(MAKE) -C wasm test
else
	@echo "Docker not installed. Skipping OPA-WASM library test."
endif

.PHONY: wasm-test
wasm-rego-test: _test/testcases.tar.gz
ifeq ($(DOCKER_INSTALLED), 1)
	@./build/run-wasm-tests.sh
else
	@echo "Docker not installed. Skipping WASM-based test execution."
endif

_test/testcases.tar.gz: $(shell find ./test/wasm/ -type f)
	go run test/wasm/cmd/testgen.go \
		--input-dir test/wasm/assets \
		--output $@

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

.PHONY: clean
clean: wasm-clean
	rm -f opa_*_*
	rm -fr _test

# The docs-% pattern target will shim to the
# makefile in ./docs
.PHONY: docs-%
docs-%:
	$(MAKE) -C docs $*

######################################################
#
# CI targets
#
######################################################

.PHONY: travis-build
travis-build: wasm-build
	@# this image is used in `Dockerfile` for image-quick
	$(DOCKER) build -t build-$(BUILD_COMMIT) --build-arg GOVERSION=$(GOVERSION) -f Dockerfile.build .
	@# the '/.' means "don't create the directory, copy its content only"
	@# these are copied our to be used from the s3 upload targets
	@# note: we don't bother cleaning up the container created here
	$(DOCKER) cp "$$($(DOCKER) create build-$(BUILD_COMMIT)):/out/." .

.PHONY: travis-test
travis-test: travis-build wasm-test wasm-rego-test
	$(DOCKER) run build-$(BUILD_COMMIT) make go-test perf check

.PHONY: travis-all
travis-all: travis-test docker-fuzzit-local-regression

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
	$(DOCKER) build --build-arg BUILD_COMMIT=$(BUILD_COMMIT) -t $(IMAGE):$(VERSION) .
	$(DOCKER) build --build-arg BUILD_COMMIT=$(BUILD_COMMIT) -t $(IMAGE):$(VERSION)-debug --build-arg VARIANT=:debug .
	$(DOCKER) build --build-arg BUILD_COMMIT=$(BUILD_COMMIT) -t $(IMAGE):$(VERSION)-rootless --build-arg USER=1 .

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
release-travis: push-image tag-latest push-latest

.PHONY: release-bugfix-travis
release-bugfix-travis: deploy-travis

.PHONY: fuzzer
fuzzer:
	docker build \
		-f Dockerfile.fuzzit \
		-t openpolicyagent/fuzzer:$(BUILD_COMMIT) \
		--build-arg GOVERSION=$(GOVERSION) \
		.

.PHONY: docker-fuzzit-local-regression
docker-fuzzit-local-regression: fuzzer
	docker run \
		-v /var/run/docker.sock:/var/run/docker.sock \
		openpolicyagent/fuzzer:$(BUILD_COMMIT) \
		./fuzzit create job --type "local-regression" opa/ast ast-fuzzer

.PHONY: docker-fuzzit-fuzzing
docker-fuzzit-fuzzing: fuzzer
	@docker run \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-e FUZZIT_API_KEY=$(FUZZIT_API_KEY) \
		openpolicyagent/fuzzer:$(BUILD_COMMIT) \
		./fuzzit create job --type "fuzzing" opa/ast ast-fuzzer

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
