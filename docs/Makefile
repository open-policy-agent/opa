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

.PHONY: generate-cli-docs
generate-cli-docs:
	$(CURDIR)/../build/gen-cli-docs.sh "$(CURDIR)/content"

.PHONY: generate
generate: generate-cli-docs copy-hugo-static-content
	$(CURDIR)/website/scripts/load-docs.sh

.PHONY: copy-hugo-static-content
copy-hugo-static-content:
	mkdir -p $(CURDIR)/website/generated/
	cp -r $(CURDIR)/website/content/* $(CURDIR)/website/generated/

.PHONY: dev-generate
dev-generate: generate-cli-docs copy-hugo-static-content
	DEV=true $(CURDIR)/website/scripts/load-docs.sh

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
dev-build: clean dev-generate hugo-production-build

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

.PHONY: preview-build
preview-build:
	hugo \
		--source $(CURDIR)/website \
		--contentDir generated \
		--baseURL $(DEPLOY_PRIME_URL) \
		--buildDrafts \
		--buildFuture \
		--ignoreCache

.PHONY: new-site
new-site:
	cd new && \
		npm ci && \
		npx docusaurus build && \
		mkdir -p ../website/public/new && \
		cp -r build/. ../website/public/new
