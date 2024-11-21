disco.tar.gz: bundle/data.json
	opa build bundle
	mv bundle.tar.gz disco.tar.gz

v1bundle.tar.gz: v1bundle/.manifest v1bundle/policy.rego
	opa build --v1-compatible -o v1bundle.tar.gz -b v1bundle/

