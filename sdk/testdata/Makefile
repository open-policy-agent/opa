disco.tar.gz: bundle/data.json
	opa build bundle
	mv bundle.tar.gz disco.tar.gz

policy1.tar.gz:
	opa build --partial-namespace policy1_partial --optimize=1 --v1-compatible -o policy1.tar.gz -b policy1/

policy2.tar.gz:
	opa build --partial-namespace policy2_partial --optimize=1 --v1-compatible -o policy2.tar.gz -b policy2/
