module github.com/open-policy-agent/opa/build/tools

go 1.25.7

tool (
	github.com/josephspurrier/goversioninfo/cmd/goversioninfo
	github.com/rogpeppe/go-internal/cmd/testscript
	golang.org/x/vuln/cmd/govulncheck
)

require (
	github.com/akavel/rsrc v0.10.2 // indirect
	github.com/josephspurrier/goversioninfo v1.7.0 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	golang.org/x/mod v0.35.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/telemetry v0.0.0-20260421165255-392afab6f40e // indirect
	golang.org/x/tools v0.44.0 // indirect
	golang.org/x/vuln v1.3.0 // indirect
)
