module github.com/open-policy-agent/opa/build/tools

go 1.25.7

tool (
	github.com/josephspurrier/goversioninfo/cmd/goversioninfo
	github.com/rogpeppe/go-internal/cmd/testscript
	golang.org/x/perf/cmd/benchstat
	golang.org/x/vuln/cmd/govulncheck
	rsc.io/cmd/benchlab
)

require (
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/aclements/go-moremath v0.0.0-20210112150236-f10218a38794 // indirect
	github.com/akavel/rsrc v0.10.2 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/josephspurrier/goversioninfo v1.7.0 // indirect
	github.com/mattn/go-runewidth v0.0.23 // indirect
	github.com/rogpeppe/go-internal v1.15.0 // indirect
	github.com/vbauerster/mpb/v8 v8.12.1 // indirect
	golang.org/x/mod v0.35.0 // indirect
	golang.org/x/perf v0.0.0-20260512194132-3cf34090a3db // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.44.0 // indirect
	golang.org/x/telemetry v0.0.0-20260421165255-392afab6f40e // indirect
	golang.org/x/tools v0.44.0 // indirect
	golang.org/x/vuln v1.3.0 // indirect
	rsc.io/cmd/benchlab v0.0.0-20260520161042-9fc40f0f0431 // indirect
)

replace rsc.io/cmd/benchlab => github.com/FiloSottile/rsc-cmd/benchlab v0.0.0-20260520161042-9fc40f0f0431
