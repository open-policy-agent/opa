# Copyright 2016 The OPA Authors.  All rights reserved.
# Use of this source code is governed by an Apache2
# license that can be found in the LICENSE file.

VERSION := 0.8.1

PACKAGES := $(shell go list ./.../ | grep -v 'vendor')

GO := go
GOARCH := $(shell go env GOARCH)
GOOS := $(shell go env GOOS)
DISABLE_CGO := CGO_ENABLED=0

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

.PHONY: all build check check-fmt check-lint check-vet \
	clean cover deps docs fmt generate install perf perf-regression \
	push push-latest push-release-builder release release-builder \
	release-patch tag-latest test version

######################################################
#
# Development targets
#
######################################################

all: deps build test check

version:
	@echo $(VERSION)

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
	sed -e 's/GOARCH/$(GOARCH)/g' Dockerfile_alpine.in > .Dockerfile_alpine_$(GOARCH)
	docker build -t $(IMAGE):$(VERSION)	-f .Dockerfile_$(GOARCH) .
	docker build -t $(IMAGE):$(VERSION)-alpine -f .Dockerfile_alpine_$(GOARCH) .

push:
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):$(VERSION)-alpine

tag-latest:
	docker tag $(IMAGE):$(VERSION) $(IMAGE):latest
	docker tag $(IMAGE):$(VERSION)-alpine $(IMAGE):latest-alpine

push-latest:
	docker push $(IMAGE):latest
	docker push $(IMAGE):latest-alpine

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
	rm -f .Dockerfile_*
	rm -f opa_*_*
	rm -fr site.tar.gz docs/_site docs/node_modules docs/book/_book docs/book/node_modules

docs:
	docker run -it --rm \
		-v $(PWD):/go/src/github.com/open-policy-agent/opa \
		-w /go/src/github.com/open-policy-agent/opa \
		-p 4000:4000 \
		$(REPOSITORY)/release-builder:latest \
		./build/build-docs.sh --output-dir=/go/src/github.com/open-policy-agent/opa --serve=4000

######################################################
#
# Release targets
#
######################################################

release-builder:
	sed -e s/GOVERSION/$(shell python -c 'import yaml; print yaml.load(open("./.travis.yml"))["go"][0]')/g Dockerfile_release-builder.in > .Dockerfile_release-builder
	docker build -f .Dockerfile_release-builder -t $(REPOSITORY)/release-builder .

push-release-builder:
	docker push $(REPOSITORY)/release-builder

release:
	docker run -it --rm \
		-v $(PWD)/_release/$(VERSION):/_release/$(VERSION) \
		-v $(PWD):/_src \
		$(REPOSITORY)/release-builder:latest \
		/_src/build/build-release.sh --version=$(VERSION) --output-dir=/_release/$(VERSION) --source-url=/_src

release-local:
	docker run -it --rm \
		-v $(PWD)/_release/$(VERSION):/_release/$(VERSION) \
		-v $(PWD):/_src \
		$(REPOSITORY)/release-builder:latest \
		/_src/build/build-release.sh --output-dir=/_release/$(VERSION) --source-url=/_src

release-patch:
	@docker run -it --rm \
		-v $(PWD):/_src \
		python:2.7 \
		/_src/build/gen-release-patch.sh --version=$(VERSION) --source-url=/_src

dev-patch:
	@docker run -it --rm \
		-v $(PWD):/_src \
		python:2.7 \
		/_src/build/gen-dev-patch.sh --version=$(VERSION) --source-url=/_src
