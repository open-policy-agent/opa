# Copyright 2016 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

VERSION := 0.8.3-dev

PACKAGES := $(shell go list ./.../ | grep -v 'vendor')

GO := go
GOVERSION := 1.10
GOARCH := $(shell go env GOARCH)
GOOS := $(shell go env GOOS)
DISABLE_CGO := CGO_ENABLED=0

BIN := opa_$(GOOS)_$(GOARCH)

REPOSITORY := openpolicyagent
IMAGE := $(REPOSITORY)/opa

BUILD_COMMIT := $(shell ./build/get-build-commit.sh)
BUILD_TIMESTAMP := $(shell ./build/get-build-timestamp.sh)
BUILD_HOSTNAME := $(shell ./build/get-build-hostname.sh)

RELEASE_BUILDER_VERSION := 1.0

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
all: deps build test check

.PHONY: version
version:
	@echo $(VERSION)

.PHONY: deps
deps:
	@./build/install-deps.sh

.PHONY: generate
generate:
	$(GO) generate

.PHONY: build
build: generate
	$(DISABLE_CGO) $(GO) build -o $(BIN) -ldflags $(LDFLAGS)

.PHONY: image
image:
	@$(MAKE) build GOOS=linux
	@$(MAKE) image-quick

.PHONY: image-quick
image-quick:
	sed -e 's/GOARCH/$(GOARCH)/g' Dockerfile.in > .Dockerfile_$(GOARCH)
	sed -e 's/GOARCH/$(GOARCH)/g' Dockerfile_alpine.in > .Dockerfile_alpine_$(GOARCH)
	docker build -t $(IMAGE):$(VERSION)	-f .Dockerfile_$(GOARCH) .
	docker build -t $(IMAGE):$(VERSION)-alpine -f .Dockerfile_alpine_$(GOARCH) .

.PHONY: push
push:
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):$(VERSION)-alpine

.PHONY: tag-latest
tag-latest:
	docker tag $(IMAGE):$(VERSION) $(IMAGE):latest
	docker tag $(IMAGE):$(VERSION)-alpine $(IMAGE):latest-alpine

.PHONY: push-latest
push-latest:
	docker push $(IMAGE):latest
	docker push $(IMAGE):latest-alpine

.PHONY: install
install: generate
	$(DISABLE_CGO) $(GO) install -ldflags $(LDFLAGS)

.PHONY: test
test: generate
	$(DISABLE_CGO) $(GO) test $(PACKAGES)

COVER_PACKAGES=$(PACKAGES)
$(COVER_PACKAGES):
	@mkdir -p coverage/$(shell dirname $@)
	$(DISABLE_CGO) $(GO) test -covermode=count -coverprofile=coverage/$(shell dirname $@)/coverage.out $@
	$(GO) tool cover -html=coverage/$(shell dirname $@)/coverage.out || true

.PHONY: perf
perf: generate
	$(DISABLE_CGO) $(GO) test -v -run=donotruntests -bench=. $(PACKAGES) | grep "^Benchmark"

.PHONY: perf-regression
perf-regression:
	./build/run-perf-regression.sh

.PHONY: cover
cover: $(COVER_PACKAGES)

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
	$(GO) fmt $(PACKAGES)

.PHONY: clean
clean:
	rm -f .Dockerfile_*
	rm -f opa_*_*
	rm -fr site.tar.gz docs/_site docs/node_modules docs/book/_book docs/book/node_modules

.PHONY: docs
docs:
	docker run -it --rm \
		-v $(PWD):/go/src/github.com/open-policy-agent/opa \
		-w /go/src/github.com/open-policy-agent/opa \
		-p 4000:4000 \
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
		-v $(PWD):/_src \
		python:2.7 \
		/_src/build/gen-release-patch.sh --version=$(VERSION) --source-url=/_src

.PHONY: dev-patch
dev-patch:
	@docker run -it --rm \
		-v $(PWD):/_src \
		python:2.7 \
		/_src/build/gen-dev-patch.sh --version=$(VERSION) --source-url=/_src
