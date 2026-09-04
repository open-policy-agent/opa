#!/bin/sh

set -e

# A generator is either a .go file in this module or a tool pinned in
# build/tools. Both run in the caller's working directory, so relative paths
# in //go:generate directives still resolve.
case $1 in
*.go)
	GOOS= GOARCH= CC= go run -tags generate "$@"
	;;
*)
	pkg=$1
	shift

	root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
	bin=$root/build/tools/bin/$(basename "$pkg")

	GOOS= GOARCH= CC= go -C "$root/build/tools" build -o "$bin" "$pkg"

	exec "$bin" "$@"
	;;
esac
