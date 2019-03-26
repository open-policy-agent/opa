load-docs:
	scripts/load-docs.sh

setup:
	npm install

serve: load-docs
	hugo server \
		--buildDrafts \
		--buildFuture

production-build: load-docs
	hugo --minify

preview-build: load-docs
	hugo \
		--baseURL $(DEPLOY_PRIME_URL) \
		--buildDrafts \
		--buildFuture \
		--minify
