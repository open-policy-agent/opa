#!/usr/bin/env bash

set -ex

PROGRAM=$(basename $0)

function usage() {
	echo ""
	echo "$0 <arguments> [options]"
	echo ""
	echo "Runs 'go build ...' with flags needed to produce binaries for"
	echo "other platforms, e.g., linux, darwin, windows, etc."
	echo ""
	echo "arguments:"
	echo ""
	echo "  --prefix <prefix> (e.g., 'opa')"
	echo "  --platforms <GOOS_1>/<GOARCH_1> <GOOS_2>/<GOARCH_2> ... (e.g., 'linux/amd64 darwin/amd64')"
	echo ""
	echo "options:"
	echo ""
	echo "  --ldflags <ldflags> (e.g., '-X ...')"
	echo ""
}

while [[ $# -gt 0 ]]; do
	key=$1
	case $key in
		--prefix)
		PREFIX="$2"
		shift
		;;
		--ldflags)
		LDFLAGS="$2"
		shift
		;;
		--platforms)
		PLATFORMS="$2"
		shift
		;;
		-h|--help)
		usage
		exit 0
		;;
		*)
		;;
	esac
shift
done

if [[ -z "$PREFIX" || -z "$PLATFORMS" ]]; then
	usage
	exit 1
fi


for x in $PLATFORMS; do
	IFS='/' read -a platform <<< "$x"
	GOOS=${platform[0]}
	GOARCH=${platform[1]}
	env GOOS=$GOOS GOARCH=$GOARCH go build -o ${PREFIX}_${GOOS}_${GOARCH} -ldflags "${LDFLAGS}"
done
