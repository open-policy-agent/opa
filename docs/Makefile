setup:
	npm install

serve:
	hugo server \
		--buildDrafts \
		--buildFuture

production-build:
	hugo --minify

preview-build:
	hugo \
		--baseURL $(DEPLOY_PRIME_URL) \
		--buildDrafts \
		--buildFuture \
		--minify
