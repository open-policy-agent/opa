DEPLOY_PRIME_URL ?= "http://localhost:8888"

######################################################
#
# Development targets
#
######################################################

.PHONY: clean
clean:
	rm -f $(CURDIR)/website/data/releases.yaml
	rm -f $(CURDIR)/website/data/builtin_metadata.json
	rm -rf $(CURDIR)/website/generated
	rm -rf $(CURDIR)/website/public
	rm -rf $(CURDIR)/website/resources
	rm -rf $(CURDIR)/website/scripts/live-blocks/opa_versions/latest-*

.PHONY: generate-cli-docs
generate-cli-docs:
	$(CURDIR)/../build/gen-cli-docs.sh "$(CURDIR)/content"

.PHONY: generate
generate: generate-cli-docs copy-builtin-metadata
	$(CURDIR)/website/scripts/load-docs.sh

.PHONY: copy-builtin-metadata
copy-builtin-metadata:
	cp -v $(CURDIR)/../builtin_metadata.json $(CURDIR)/website/data/builtin_metadata.json

.PHONY: dev-generate
dev-generate: generate-cli-docs copy-builtin-metadata
	DEV=true $(CURDIR)/website/scripts/load-docs.sh

# The website has some npm dependencies saved in ./website/node_modules
# As well as dependencies required for the "live-blocks".
# Use this target to update them as needed.
.PHONY: install-deps
install-deps: live-blocks-install-deps
	npm install

# The live-blocks-% pattern target will shim to the
# npm scripts in ./website/scripts/live-blocks
# NOTE(sr): we'll avoid an API call if we make use of releases.yaml to determine the latest release
.PHONY: live-blocks-%
live-blocks-%:
	cd $(CURDIR)/website/scripts/live-blocks && LATEST=$(shell sed -n '2s/- //p' $(CURDIR)/website/data/releases.yaml) npm run $*

.PHONY: serve-local
serve-local: dev-build
	# must be run from root of repo for
	# the netlify.toml config to work
	cd $(CURDIR)/.. && netlify dev --offline --dir $(CURDIR)/website/public

.PHONY: serve-remote
serve-remote: production-build
	# must be run from root of repo for
	# the netlify.toml config to work
	cd $(CURDIR)/.. && netlify deploy

.PHONY: dev-build
dev-build: clean dev-generate copy-builtin-metadata hugo-production-build live-blocks-inject

######################################################
#
# CI targets
#
######################################################

.PHONY: hugo-production-build
hugo-production-build:
	hugo \
		--source $(CURDIR)/website \
		--contentDir generated \
		--ignoreCache \
		--minify

.PHONY: production-build
production-build: clean generate hugo-production-build
	make live-blocks-inject

.PHONY: preview-build
preview-build:
	hugo \
		--source $(CURDIR)/website \
		--contentDir generated \
		--baseURL $(DEPLOY_PRIME_URL) \
		--buildDrafts \
		--buildFuture \
		--ignoreCache
	make live-blocks-inject
