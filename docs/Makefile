.PHONY: install
install:
	npm install

.PHONY: ci
ci:
	npm ci

.PHONY: dev
dev:
	# Build once and index with Pagefind so search works in dev, but skip
	# it if a build and index already exist. Run `make clean` first to
	# force a fresh build/reindex.
	if [ ! -d build ] || [ ! -d static/pagefind ]; then \
		npx docusaurus build && \
		npx pagefind --site build && \
		rm -rf static/pagefind && \
		cp -r build/pagefind static/pagefind; \
	fi
	# --no-open means that the browser will not be opened on start.
	# This is done to avoid opening many tabs repeatedly when editing
	# docusaurus.config.js.
	npx docusaurus start --no-open

.PHONY: build
build:
	npx docusaurus build
	npx pagefind --site build

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

.PHONY: fmt
fmt:
	npx dprint fmt

.PHONY: fmt-check
fmt-check:
	npx dprint check

.PHONY: lint
lint:
	npx eslint --fix .

.PHONY: lint-check
lint-check:
	npx eslint .

.PHONY: markdownlint
markdownlint:
	npx markdownlint-cli2 --fix

.PHONY: markdownlint-check
markdownlint-check:
	npx markdownlint-cli2

.PHONY: spell-check
spell-check:
	vale --config=.vale.ini .

.PHONY: gen
gen:
	./bin/eval-examples.sh

.PHONY: gen-check
gen-check:
	./bin/eval-examples.sh
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "Error: working directory is not clean after running gen"; \
		git diff; \
		exit 1; \
	fi
