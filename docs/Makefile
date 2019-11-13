DEPLOY_PRIME_URL ?= "http://localhost:8888"

.PHONY: clean
clean:
	rm -rf ${CURDIR}/website/data/releases.yaml
	rm -rf $(CURDIR)/website/generated
	rm -rf $(CURDIR)/website/public
	rm -rf $(CURDIR)/website/resources

.PHONY: generate
generate:
	$(CURDIR)/website/scripts/load-docs.sh

.PHONY: setup
setup:
	npm install

.PHONY: hugo-serve
hugo-serve:
	hugo \
		server \
		--source $(CURDIR)/website \
		--contentDir generated \
		--buildDrafts \
		--buildFuture

.PHONY: serve
serve: production-build
	# must be run from root of repo for
	# the netlify.toml config to work
	cd $(CURDIR)/.. && netlify dev --offline

.PHONY: docker-serve
docker-serve:
	docker run --rm -it \
		-v $(CURDIR)/:/src \
		-p 1313:1313 \
		klakegg/hugo:0.55.5-ext server \
			--source /src/website \
			--contentDir generated

.PHONY: hugo-production-build
hugo-production-build:
	hugo \
		--source $(CURDIR)/website \
		--contentDir generated \
		--ignoreCache \
		--minify

.PHONY: production-build
production-build: clean generate hugo-production-build live-blocks-inject

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

# The live-blocks-% pattern target will shim to the
# npm scripts in ./website/scripts/live-blocks
.PHONY: live-blocks-%
live-blocks-%:
	cd $(CURDIR)/website/scripts/live-blocks && npm run --silent $*
