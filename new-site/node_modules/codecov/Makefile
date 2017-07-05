REPORTER = spec
test:
	@$(MAKE) lint
	@NODE_ENV=test ./node_modules/.bin/mocha -b --reporter $(REPORTER) --recursive

lint:
	./node_modules/.bin/jshint ./lib ./test ./index.js

deploy:
	$(eval VERSION := $(shell cat package.json | grep '"version"' | cut -d\" -f4))
	git tag v$(VERSION) -m ""
	git push origin v$(VERSION)
	npm publish

testsuite:
	curl -X POST https://circleci.com/api/v1/project/codecov/testsuite/tree/master?circle-token=$(CIRCLE_TOKEN)\
	     --header "Content-Type: application/json"\
	     --data "{\"build_parameters\": {\"TEST_LANG\": \"node\",\
	                                     \"TEST_SLUG\": \"$(CIRCLE_PROJECT_USERNAME)/$(CIRCLE_PROJECT_REPONAME)\",\
	                                     \"TEST_SHA\": \"$(CIRCLE_SHA1)\"}}"


.PHONY: test
