.PHONY: install
install:
	npm install

.PHONY: ci
ci:
	npm ci

.PHONY: dev
dev:
	# --no-open means that the browser will not be opened on start.
	# This is done to avoid opening many tabs repeatedly when editing
	# docusaurus.config.js.
	npx docusaurus start --no-open

.PHONY: build
build:
	npx docusaurus build

.PHONY: build-latest
build-latest:
	./bin/build-latest.sh

.PHONY: clean
clean:
	rm -rf build .docusaurus

.PHONY: generate-cli-docs
generate-cli-docs:
	$(CURDIR)/../build/gen-cli-docs.sh > $(CURDIR)/src/data/cli.json

.PHONY: smoke-test
smoke-test:
	./bin/smoke-test.sh
