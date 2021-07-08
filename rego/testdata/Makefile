# NOTE(sr): this is the last image emitting wasm modules with ABI 1.1
IMAGE := openpolicyagent/opa:0.30.1

bundle.tar.gz: t.rego
	docker run -v $$(pwd):/src -w /src $(IMAGE) build -t wasm -e test/allow t.rego