# Copyright 2016 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

VERSION := $(shell ./build/get-build-version.sh)

CGO_ENABLED ?= 1
WASM_ENABLED ?= 1

# See https://golang.org/cmd/go/#hdr-Build_modes:
# > -buildmode=exe
# > Build the listed main packages and everything they import into
# > executables. Packages not named main are ignored.
GO := CGO_ENABLED=$(CGO_ENABLED) GOFLAGS="-buildmode=exe" go
GO_TEST_TIMEOUT := -timeout 30m

GOVERSION ?= $(shell cat ./.go-version)
GOARCH := $(shell go env GOARCH)
GOOS := $(shell go env GOOS)

ifeq ($(GOOS)/$(GOARCH),darwin/arm64)
WASM_ENABLED=0
endif

GO_TAGS := -tags=
ifeq ($(WASM_ENABLED),1)
GO_TAGS = -tags=opa_wasm
endif

GOLANGCI_LINT_VERSION := v1.46.2

DOCKER_RUNNING ?= $(shell docker ps >/dev/null 2>&1 && echo 1 || echo 0)

# We use root because the windows build, invoked through the ci-go-build-windows
# target, installs the gcc mingw32 cross-compiler.
# For image, it's overridden, so that the built binary isn't root-owned.
DOCKER_UID ?= 0
DOCKER_GID ?= 0

ifeq ($(shell tty > /dev/null && echo 1 || echo 0), 1)
DOCKER_FLAGS := --rm -it
else
DOCKER_FLAGS := --rm
endif

DOCKER := docker

# BuildKit is required for automatic platform arg injection (see Dockerfile)
export DOCKER_BUILDKIT := 1

# Supported platforms to include in image manifest lists
DOCKER_PLATFORMS := linux/amd64
DOCKER_PLATFORMS_STATIC := linux/amd64,linux/arm64

BIN := opa_$(GOOS)_$(GOARCH)

# Optional external configuration useful for forks of OPA
DOCKER_IMAGE ?= openpolicyagent/opa
S3_RELEASE_BUCKET ?= opa-releases
FUZZ_TIME ?= 1h
TELEMETRY_URL ?= #Default empty

BUILD_HOSTNAME := $(shell ./build/get-build-hostname.sh)

RELEASE_BUILD_IMAGE := golang:$(GOVERSION)

RELEASE_DIR ?= _release/$(VERSION)

ifneq (,$(TELEMETRY_URL))
TELEMETRY_FLAG := -X github.com/open-policy-agent/opa/internal/report.ExternalServiceURL=$(TELEMETRY_URL)
endif

LDFLAGS := "$(TELEMETRY_FLAG) \
	-X github.com/open-policy-agent/opa/version.Hostname=$(BUILD_HOSTNAME)"


######################################################
#
# Development targets
#
######################################################

# If you update the 'all' target make sure the 'ci-release-test' target is consistent.
.PHONY: all
all: build test perf wasm-sdk-e2e-test check

.PHONY: version
version:
	@echo $(VERSION)

.PHONY: release-dir
release-dir:
	@echo $(RELEASE_DIR)

.PHONY: generate
generate: wasm-lib-build
	$(GO) generate

.PHONY: build
build: go-build

.PHONY: image
image:
	DOCKER_UID=$(shell id -u) DOCKER_GID=$(shell id -g) $(MAKE) ci-go-ci-build-linux ci-go-ci-build-linux-static
	@$(MAKE) image-quick

.PHONY: install
install: generate
	$(GO) install $(GO_TAGS) -ldflags $(LDFLAGS)

.PHONY: test
test: go-test wasm-test

.PHONY: go-build
go-build: generate
	$(GO) build $(GO_TAGS) -o $(BIN) -ldflags $(LDFLAGS)

.PHONY: go-test
go-test: generate
	$(GO) test $(GO_TAGS),slow ./...

.PHONY: race-detector
race-detector: generate
	$(GO) test $(GO_TAGS),slow -race -vet=off ./...

.PHONY: test-coverage
test-coverage: generate
	$(GO) test $(GO_TAGS),slow -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: perf
perf: generate
	$(GO) test $(GO_TAGS),slow $(GO_TEST_TIMEOUT) -run=- -bench=. -benchmem ./...

.PHONY: perf-noisy
perf-noisy: generate
	$(GO) test $(GO_TAGS),slow,noisy $(GO_TEST_TIMEOUT) -run=- -bench=. -benchmem ./...

.PHONY: wasm-sdk-e2e-test
wasm-sdk-e2e-test: generate
	$(GO) test $(GO_TAGS),slow,wasm_sdk_e2e $(GO_TEST_TIMEOUT) -v ./internal/wasm/sdk/test/e2e

.PHONY: check
check:
ifeq ($(DOCKER_RUNNING), 1)
	docker run --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:${GOLANGCI_LINT_VERSION} golangci-lint run -v
else
	@echo "Docker not installed or running. Skipping golangci run."
endif

.PHONY: fmt
fmt:
ifeq ($(DOCKER_RUNNING), 1)
	docker run --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:${GOLANGCI_LINT_VERSION} golangci-lint run -v --fix
else
	@echo "Docker not installed or running. Skipping golangci run."
endif

.PHONY: clean
clean: wasm-lib-clean
	rm -f opa_*_*

.PHONY: fuzz
fuzz:
	go test ./ast -fuzz FuzzParseStatementsAndCompileModules -fuzztime ${FUZZ_TIME} -v -run '^$$'

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
ifeq ($(DOCKER_RUNNING), 1)
	@$(MAKE) -C wasm ensure-builder build
	cp wasm/_obj/opa.wasm internal/compiler/wasm/opa/opa.wasm
	cp wasm/_obj/callgraph.csv internal/compiler/wasm/opa/callgraph.csv
else
	@echo "Docker not installed or not running. Skipping OPA-WASM library build."
endif

.PHONY: wasm-lib-test
wasm-lib-test:
ifeq ($(DOCKER_RUNNING), 1)
	@$(MAKE) -C wasm ensure-builder test
else
	@echo "Docker not installed or not running. Skipping OPA-WASM library test."
endif

.PHONY: wasm-rego-test
wasm-rego-test: generate
ifeq ($(DOCKER_RUNNING), 1)
	GOVERSION=$(GOVERSION) ./build/run-wasm-rego-tests.sh
else
	@echo "Docker not installed or not running. Skipping Rego-WASM test."
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

CI_GOLANG_DOCKER_MAKE := $(DOCKER) run \
	$(DOCKER_FLAGS) \
	-u $(DOCKER_UID):$(DOCKER_GID) \
	-v $(PWD):/src \
	-w /src \
	-e GOCACHE=/src/.go/cache \
	-e GOARCH=$(GOARCH) \
	-e CGO_ENABLED=$(CGO_ENABLED) \
	-e WASM_ENABLED=$(WASM_ENABLED) \
	-e FUZZ_TIME=$(FUZZ_TIME) \
	-e TELEMETRY_URL=$(TELEMETRY_URL) \
	golang:$(GOVERSION) \
	make

.PHONY: ci-go-%
ci-go-%: generate
	$(CI_GOLANG_DOCKER_MAKE) $*

.PHONY: ci-release-test
ci-release-test: generate
	$(CI_GOLANG_DOCKER_MAKE) test perf wasm-sdk-e2e-test check

.PHONY: ci-check-working-copy
ci-check-working-copy: generate
	./build/check-working-copy.sh

.PHONY: ci-wasm
ci-wasm: wasm-test

.PHONY: ci-build-linux
ci-build-linux: ensure-release-dir ensure-linux-toolchain
	@$(MAKE) build GOOS=linux
	chmod +x opa_linux_$(GOARCH)
	mv opa_linux_$(GOARCH) $(RELEASE_DIR)/
	cd $(RELEASE_DIR)/ && shasum -a 256 opa_linux_$(GOARCH) > opa_linux_$(GOARCH).sha256

.PHONY: ci-build-linux-static
ci-build-linux-static: ensure-release-dir
	@$(MAKE) build GOOS=linux WASM_ENABLED=0 CGO_ENABLED=0
	chmod +x opa_linux_$(GOARCH)
	mv opa_linux_$(GOARCH) $(RELEASE_DIR)/opa_linux_$(GOARCH)_static
	cd $(RELEASE_DIR)/ && shasum -a 256 opa_linux_$(GOARCH)_static > opa_linux_$(GOARCH)_static.sha256

.PHONY: ci-build-darwin
ci-build-darwin: ensure-release-dir
	@$(MAKE) build GOOS=darwin
	chmod +x opa_darwin_$(GOARCH)
	mv opa_darwin_$(GOARCH) $(RELEASE_DIR)/
	cd $(RELEASE_DIR)/ && shasum -a 256 opa_darwin_$(GOARCH) > opa_darwin_$(GOARCH).sha256

.PHONY: ci-build-darwin-arm64-static
ci-build-darwin-arm64-static: ensure-release-dir
	@$(MAKE) build GOOS=darwin GOARCH=arm64 WASM_ENABLED=0 CGO_ENABLED=0
	chmod +x opa_darwin_arm64
	mv opa_darwin_arm64 $(RELEASE_DIR)/opa_darwin_arm64_static
	cd $(RELEASE_DIR)/ && shasum -a 256 opa_darwin_arm64_static > opa_darwin_arm64_static.sha256

# NOTE: This target expects to be run as root on some debian/ubuntu variant
# that can install the `gcc-mingw-w64-x86-64` package via apt-get.
.PHONY: ci-build-windows
ci-build-windows: ensure-release-dir
	build/ensure-windows-toolchain.sh
	@$(MAKE) build GOOS=windows CC=x86_64-w64-mingw32-gcc
	mv opa_windows_$(GOARCH) $(RELEASE_DIR)/opa_windows_$(GOARCH).exe
	cd $(RELEASE_DIR)/ && shasum -a 256 opa_windows_$(GOARCH).exe > opa_windows_$(GOARCH).exe.sha256

.PHONY: ensure-release-dir
ensure-release-dir:
	mkdir -p $(RELEASE_DIR)

.PHONY: ensure-executable-bin
ensure-executable-bin:
	find $(RELEASE_DIR) -type f ! -name "*.sha256" | xargs chmod +x

.PHONY: ensure-linux-toolchain
ensure-linux-toolchain:
ifeq ($(CGO_ENABLED),1)
	$(eval export CC = $(shell GOARCH=$(GOARCH) build/ensure-linux-toolchain.sh))
else
	@echo "CGO_ENABLED=$(CGO_ENABLED). No need to check gcc toolchain."
endif

.PHONY: build-all-platforms
build-all-platforms: ci-build-linux ci-build-linux-static ci-build-darwin ci-build-darwin-arm64-static ci-build-windows

.PHONY: image-quick
image-quick: image-quick-$(GOARCH)

# % = arch
.PHONY: image-quick-%
image-quick-%: ensure-executable-bin
ifneq ($(GOARCH),arm64) # build only static images for arm64
	$(DOCKER) build \
		-t $(DOCKER_IMAGE):$(VERSION) \
		--build-arg BASE=gcr.io/distroless/cc \
		--build-arg BIN_DIR=$(RELEASE_DIR) \
		--platform linux/$* \
		.
	$(DOCKER) build \
		-t $(DOCKER_IMAGE):$(VERSION)-debug \
		--build-arg BASE=gcr.io/distroless/cc:debug \
		--build-arg BIN_DIR=$(RELEASE_DIR) \
		--platform linux/$* \
		.
	$(DOCKER) build \
		-t $(DOCKER_IMAGE):$(VERSION)-rootless \
		--build-arg USER=1000:1000 \
		--build-arg BASE=gcr.io/distroless/cc \
		--build-arg BIN_DIR=$(RELEASE_DIR) \
		--platform linux/$* \
		.
endif
	$(DOCKER) build \
		-t $(DOCKER_IMAGE):$(VERSION)-static \
		--build-arg BASE=gcr.io/distroless/static \
		--build-arg BIN_DIR=$(RELEASE_DIR) \
		--build-arg BIN_SUFFIX=_static \
		--platform linux/$* \
		.

# % = base tag
.PHONY: push-manifest-list-%
push-manifest-list-%: ensure-executable-bin
	$(DOCKER) buildx build \
		--tag $(DOCKER_IMAGE):$* \
		--build-arg BASE=gcr.io/distroless/cc \
		--build-arg BIN_DIR=$(RELEASE_DIR) \
		--platform $(DOCKER_PLATFORMS) \
		--push \
		.
	$(DOCKER) buildx build \
		--tag $(DOCKER_IMAGE):$*-debug \
		--build-arg BASE=gcr.io/distroless/cc:debug \
		--build-arg BIN_DIR=$(RELEASE_DIR) \
		--platform $(DOCKER_PLATFORMS) \
		--push \
		.
	$(DOCKER) buildx build \
		--tag $(DOCKER_IMAGE):$*-rootless \
		--build-arg USER=1000:1000 \
		--build-arg BASE=gcr.io/distroless/cc \
		--build-arg BIN_DIR=$(RELEASE_DIR) \
		--platform $(DOCKER_PLATFORMS) \
		--push \
		.
	$(DOCKER) buildx build \
		--tag $(DOCKER_IMAGE):$*-static \
		--build-arg BASE=gcr.io/distroless/static \
		--build-arg BIN_DIR=$(RELEASE_DIR) \
		--build-arg BIN_SUFFIX=_static \
		--platform $(DOCKER_PLATFORMS_STATIC) \
		--push \
		.

.PHONY: ci-image-smoke-test
ci-image-smoke-test: ci-image-smoke-test-$(GOARCH)

# % = arch
.PHONY: ci-image-smoke-test-%
ci-image-smoke-test-%: image-quick-%
ifneq ($(GOARCH),arm64) # we build only static images for arm64
	$(DOCKER) run --platform linux/$* $(DOCKER_IMAGE):$(VERSION) version
	$(DOCKER) run --platform linux/$* $(DOCKER_IMAGE):$(VERSION)-debug version
	$(DOCKER) run --platform linux/$* $(DOCKER_IMAGE):$(VERSION)-rootless version

	$(DOCKER) image inspect $(DOCKER_IMAGE):$(VERSION)-rootless | opa eval --fail --format raw --stdin-input 'input[0].Config.User = "1000:1000"'
endif
	$(DOCKER) run --platform linux/$* $(DOCKER_IMAGE):$(VERSION)-static version

# % = rego/wasm
.PHONY: ci-binary-smoke-test-%
ci-binary-smoke-test-%:
	chmod +x "$(RELEASE_DIR)/$(BINARY)"
	"$(RELEASE_DIR)/$(BINARY)" version
	"$(RELEASE_DIR)/$(BINARY)" eval -t "$*" 'time.now_ns()'

.PHONY: push-binary-edge
push-binary-edge:
	aws s3 sync $(RELEASE_DIR) s3://$(S3_RELEASE_BUCKET)/edge/ --no-progress --region us-west-1

.PHONY: docker-login
docker-login:
	@echo "Docker Login..."
	@echo ${DOCKER_PASSWORD} | $(DOCKER) login -u ${DOCKER_USER} --password-stdin

.PHONY: push-image
push-image: docker-login push-manifest-list-$(VERSION)

.PHONY: push-wasm-builder-image
push-wasm-builder-image: docker-login
	$(MAKE) -C wasm push-builder

.PHONY: deploy-ci
deploy-ci: push-image push-manifest-list-edge push-binary-edge

.PHONY: release-ci
# Don't tag and push "latest" image tags if the version is a release candidate or a bugfix branch
# where the changes don't exist in main
ifneq (,$(or $(findstring rc,$(VERSION)), $(findstring release-,$(shell git branch --contains HEAD))))
release-ci: push-image
else
release-ci: push-image push-manifest-list-latest
endif

.PHONY: netlify-prod
netlify-prod: clean docs-clean build docs-production-build

.PHONY: netlify-preview
netlify-preview: clean docs-clean build docs-live-blocks-install-deps docs-live-blocks-test docs-dev-generate docs-preview-build

# Kept for compatibility. Use `make fuzz` instead.
.PHONY: check-fuzz
check-fuzz: fuzz

# GOPRIVATE=* causes go to fetch all dependencies from their corresponding VCS
# source, not through the golang-provided proxy services. We're cleaning out
# /src/.go by providing a tmpfs mount, so the `go mod vendor -v` command will
# not be able to use any module cache.
.PHONY: check-go-module
check-go-module:
	docker run \
	  $(DOCKER_FLAGS) \
	  -w /src \
	  -v $(PWD):/src \
	  -e 'GOPRIVATE=*' \
	  --tmpfs /src/.go \
	  golang:$(GOVERSION) \
	  go mod vendor -v

######################################################
#
# Release targets
#
######################################################

.PHONY: release-patch
release-patch:
ifeq ($(GITHUB_TOKEN),)
	@echo "\033[0;31mGITHUB_TOKEN environment variable missing.\033[33m Provide a GitHub Personal Access Token (PAT) with the 'read:org' scope.\033[0m"
endif
	@$(DOCKER) run $(DOCKER_FLAGS) \
		-e GITHUB_TOKEN=$(GITHUB_TOKEN) \
		-e LAST_VERSION=$(LAST_VERSION) \
		-v $(PWD):/_src \
		cmd.cat/make/git/go/python3/perl \
		/_src/build/gen-release-patch.sh --version=$(VERSION) --source-url=/_src

.PHONY: dev-patch
dev-patch:
	@$(DOCKER) run $(DOCKER_FLAGS) \
		-v $(PWD):/_src \
		cmd.cat/make/git/go/python3/perl \
		/_src/build/gen-dev-patch.sh --version=$(VERSION) --source-url=/_src

# Deprecated targets. To be removed.
.PHONY: build-linux depr-build-linux build-windows depr-build-windows build-darwin depr-build-darwin release release-local
build-linux: deprecation-build-linux
build-windows: deprecation-build-windows
build-darwin: deprecation-build-darwin
release: deprecation-release
release-local: deprecation-release-local

.PHONY: deprecation-%
deprecation-%:
	@echo "----------------------------------------------"
	@echo "The '$*' make target is deprecated!"
	@echo "----------------------------------------------"
	@echo "To run build for your platform, use 'make build'."
	@echo "To cross-compile for a specific platform, use the corresponding 'ci-build-*' target."
	@echo
	@$(MAKE) depr-$*

depr-build-linux: ensure-release-dir
	@$(MAKE) build GOOS=linux CGO_ENABLED=0 WASM_ENABLED=0
	mv opa_linux_$(GOARCH) $(RELEASE_DIR)/

depr-build-darwin: ensure-release-dir
	@$(MAKE) build GOOS=darwin CGO_ENABLED=0 WASM_ENABLED=0
	mv opa_darwin_$(GOARCH) $(RELEASE_DIR)/

depr-build-windows: ensure-release-dir
	@$(MAKE) build GOOS=windows CGO_ENABLED=0 WASM_ENABLED=0
	mv opa_windows_$(GOARCH) $(RELEASE_DIR)/opa_windows_$(GOARCH).exe

depr-release:
	$(DOCKER) run $(DOCKER_FLAGS) \
		-v $(PWD)/$(RELEASE_DIR):/$(RELEASE_DIR) \
		-v $(PWD):/_src \
		-e TELEMETRY_URL=$(TELEMETRY_URL) \
		$(RELEASE_BUILD_IMAGE) \
		/_src/build/build-release.sh --version=$(VERSION) --output-dir=/$(RELEASE_DIR) --source-url=/_src

depr-release-local:
	$(DOCKER) run $(DOCKER_FLAGS) \
		-v $(PWD)/$(RELEASE_DIR):/$(RELEASE_DIR) \
		-v $(PWD):/_src \
		-e TELEMETRY_URL=$(TELEMETRY_URL) \
		$(RELEASE_BUILD_IMAGE) \
		/_src/build/build-release.sh --output-dir=/$(RELEASE_DIR) --source-url=/_src
