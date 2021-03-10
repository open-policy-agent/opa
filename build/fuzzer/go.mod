module github.com/open-policy-agent/fuzz-opa

go 1.14

require (
	github.com/dvyukov/go-fuzz v0.0.0-20210103155950-6a8e9d1f2415 // indirect
	github.com/elazarl/go-bindata-assetfs v1.0.1 // indirect
	github.com/open-policy-agent/opa v0.0.0
	github.com/stephens2424/writerset v1.0.2 // indirect
	golang.org/x/mod v0.4.1 // indirect
	golang.org/x/sys v0.0.0-20210309074719-68d13333faf2 // indirect
	golang.org/x/tools v0.1.0 // indirect
)

// Point the OPA dependency to the local source
replace github.com/open-policy-agent/opa => ../../
