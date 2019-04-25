clean:
	rm -rf generated
	rm -rf public
	rm -rf resources

load-docs:
	scripts/load-docs.sh

setup:
	npm install

serve:
	hugo server --contentDir generated

docker-serve:
	docker run --rm -it \
		-v $(PWD):/src \
		-p 1313:1313 \
		klakegg/hugo:0.53-ext server \
			--contentDir /src/generated

production-build: clean load-docs
	hugo \
		--contentDir generated \
		--ignoreCache \
		--minify

preview-build: clean load-docs
	hugo \
		--contentDir generated \
		--baseURL $(DEPLOY_PRIME_URL) \
		--buildDrafts \
		--buildFuture \
		--ignoreCache \
		--minify

linkcheck-build:
	hugo \
		--contentDir generated \
		--baseURL "/" \
		--buildDrafts \
		--buildFuture

linkcheck: clean load-docs linkcheck-build
	htmlproofer ./public --empty-alt-ignore
