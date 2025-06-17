.PHONY: install
install:
	npm install

.PHONY: ci
ci:
	npm ci

.PHONY: dev
dev:
	npx docusaurus start

.PHONY: build
build:
	npx docusaurus build

.PHONY: clean
clean:
	rm -rf build .docusaurus

.PHONY: generate-cli-docs
generate-cli-docs:
	$(CURDIR)/../build/gen-cli-docs.sh > $(CURDIR)/src/data/cli.json

.PHONY: smoke-test
smoke-test:
	./bin/smoke-test.sh
