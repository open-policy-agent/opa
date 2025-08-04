.PHONY: generate realclean cover viewcover test lint check_diffs imports tidy jwx
generate: 
	@go generate
	@$(MAKE) generate-jwa generate-jwe generate-jwk generate-jws generate-jwt
	@./tools/cmd/gofmt.sh

generate-%:
	@go generate $(shell pwd -P)/$(patsubst generate-%,%,$@)

realclean:
	rm coverage.out

test-cmd:
	env TESTOPTS="$(TESTOPTS)" ./tools/test.sh

test:
	$(MAKE) test-stdlib TESTOPTS=

test-stdlib:
	$(MAKE) test-cmd TESTOPTS=

test-goccy:
	$(MAKE) test-cmd TESTOPTS="-tags jwx_goccy"

test-es256k:
	$(MAKE) test-cmd TESTOPTS="-tags jwx_es256k"

test-secp256k1-pem:
	$(MAKE) test-cmd TESTOPTS="-tags jwx_es256k,jwx_secp256k1_pem"

test-asmbase64:
	$(MAKE) test-cmd TESTOPTS="-tags jwx_asmbase64"

test-alltags:
	$(MAKE) test-cmd TESTOPTS="-tags jwx_asmbase64,jwx_goccy,jwx_es256k,jwx_secp256k1_pem"

cover-cmd:
	env MODE=cover ./tools/test.sh

cover:
	$(MAKE) cover-stdlib

cover-stdlib:
	$(MAKE) cover-cmd TESTOPTS=

cover-goccy:
	$(MAKE) cover-cmd TESTOPTS="-tags jwx_goccy"

cover-es256k:
	$(MAKE) cover-cmd TESTOPTS="-tags jwx_es256k"

cover-secp256k1-pem:
	$(MAKE) cover-cmd TESTOPTS="-tags jwx_es256k,jwx_secp256k1"

cover-asmbase64:
	$(MAKE) cover-cmd TESTOPTS="-tags jwx_asmbase64"

cover-alltags:
	$(MAKE) cover-cmd TESTOPTS="-tags jwx_asmbase64,jwx_goccy,jwx_es256k,jwx_secp256k1_pem"

smoke-cmd:
	env MODE=short ./tools/test.sh

smoke:
	$(MAKE) smoke-stdlib

smoke-stdlib:
	$(MAKE) smoke-cmd TESTOPTS=

smoke-goccy:
	$(MAKE) smoke-cmd TESTOPTS="-tags jwx_goccy"

smoke-es256k:
	$(MAKE) smoke-cmd TESTOPTS="-tags jwx_es256k"

smoke-secp256k1-pem:
	$(MAKE) smoke-cmd TESTOPTS="-tags jwx_es256k,jwx_secp256k1_pem"

smoke-alltags:
	$(MAKE) smoke-cmd TESTOPTS="-tags jwx_goccy,jwx_es256k,jwx_secp256k1_pem"

viewcover:
	go tool cover -html=coverage.out

lint:
	golangci-lint run ./...

check_diffs:
	./scripts/check-diff.sh

imports:
	goimports -w ./

tidy:
	./scripts/tidy.sh

jwx:
	@./tools/cmd/install-jwx.sh
