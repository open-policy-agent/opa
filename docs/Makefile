.PHONY: clean
clean:
	rm -rf $(CURDIR)/website/generated 
	rm -rf $(CURDIR)/website/public 
	rm -rf $(CURDIR)/website/esources

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

.PHONY: production-build
production-build: clean generate
	hugo \
		--source $(CURDIR)/website \
		--contentDir generated \
		--ignoreCache \
		--minify

.PHONY: preview-build
preview-build: clean generate
	hugo \
		--source $(CURDIR)/website \
		--contentDir generated \
		--baseURL $(DEPLOY_PRIME_URL) \
		--buildDrafts \
		--buildFuture \
		--ignoreCache \
		--minify

.PHONY: linkcheck-build
linkcheck-build:
	hugo \
		--source $(CURDIR)/website \
		--contentDir generated \
		--baseURL "/" \
		--buildDrafts \
		--buildFuture

.PHONY: linkcheck
linkcheck: clean generate linkcheck-build
	htmlproofer ./public --empty-alt-ignore
