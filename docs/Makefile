.PHONY: clean
clean:
	rm -rf generated
	rm -rf public
	rm -rf resources

.PHONY: generate
generate:
	scripts/load-docs.sh

.PHONY: setup
setup:
	npm install

.PHONY: hugo-serve
hugo-serve:
	hugo \
		server \
		--contentDir generated \
		--buildDrafts \
		--buildFuture

.PHONY: serve
serve:
	# must be run from root of repo for
	# the netlify.toml config to work
	cd .. && netlify dev --offline

.PHONY: docker-serve
docker-serve:
	docker run --rm -it \
		-v $(PWD):/src \
		-p 1313:1313 \
		klakegg/hugo:0.53-ext server \
			--contentDir /src/generated

.PHONY: production-build
production-build: clean generate
	hugo \
		--contentDir generated \
		--ignoreCache \
		--minify

.PHONY: preview-build
preview-build: clean generate
	hugo \
		--contentDir generated \
		--baseURL $(DEPLOY_PRIME_URL) \
		--buildDrafts \
		--buildFuture \
		--ignoreCache \
		--minify

.PHONY: linkcheck-build
linkcheck-build:
	hugo \
		--contentDir generated \
		--baseURL "/" \
		--buildDrafts \
		--buildFuture

.PHONY: linkcheck
linkcheck: clean generate linkcheck-build
	htmlproofer ./public --empty-alt-ignore
