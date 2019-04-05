clean:
	rm -rf public

load-docs:
	scripts/load-docs.sh

setup:
	npm install

serve:
	hugo server \
		--buildDrafts \
		--buildFuture

production-build: clean load-docs
	hugo \
		--contentDir generated \
		--ignoreCache \
		--minify

preview-build: clean load-docs
	hugo \
		--baseURL $(DEPLOY_PRIME_URL) \
		--buildDrafts \
		--buildFuture \
		--contentDir generated \
		--ignoreCache \
		--minify

linkcheck-build:
	hugo \
		--baseURL "/" \
		--buildDrafts \
		--buildFuture

linkcheck: clean linkcheck-build
	htmlproofer ./public --empty-alt-ignore
