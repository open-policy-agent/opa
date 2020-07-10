module github.com/open-policy-agent/fuzz-opa

go 1.14

require (
	github.com/dvyukov/go-fuzz v0.0.0-20200318091601-be3528f3a813 // indirect
	github.com/elazarl/go-bindata-assetfs v1.0.0 // indirect
	github.com/open-policy-agent/opa v0.0.0
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20190826022208-cac0b30c2563 // indirect
	github.com/stephens2424/writerset v1.0.2 // indirect
	golang.org/x/tools v0.0.0-20200713235242-6acd2ab80ede // indirect
	gopkg.in/yaml.v2 v2.2.8 // indirect
)

// Point the OPA dependency to the local source
replace github.com/open-policy-agent/opa => ../../
