clean:
	rm -rf public

load-docs:
	scripts/load-docs.sh

setup:
	npm install

serve:
	ENV=local hugo server \
		--buildDrafts \
		--buildFuture

docker-serve:
	docker run --rm -it \
		-v $(PWD):/src \
		-p 1313:1313 \
		-e ENV=local \
		klakegg/hugo:0.53-ext server \
			--buildDrafts \
			--buildFuture

production-build: clean load-docs
	hugo \
		--contentDir generated \
		--ignoreCache \
		--minify

preview-build: clean
	@echo "- latest" > ./data/releases.yaml
	ENV=local hugo \
		--baseURL $(DEPLOY_PRIME_URL) \
		--buildDrafts \
		--buildFuture \
		--ignoreCache \
		--minify

linkcheck-build:
	hugo \
		--baseURL "/" \
		--buildDrafts \
		--buildFuture

linkcheck: clean linkcheck-build
	htmlproofer ./public --empty-alt-ignore
