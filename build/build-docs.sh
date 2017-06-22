#!/usr/bin/env bash
# Script to build OPA static site. Assumes execution environment is release builder.

set -ex

pushd `dirname $0` >/dev/null
OPA_DIR=$(pwd -P)/..
popd > /dev/null

usage() {
    echo "build-docs.sh --output-dir=<path> [--serve=<port>]"
}

for i in "$@"; do
    case $i in
    --output-dir=*)
        OUTPUT_DIR="${i#*=}"
        shift
        ;;
    --serve=*)
        PORT="${i#*=}"
        shift
        ;;
    *)
        usage
        exit 1
    esac
done

if [ -z "$OUTPUT_DIR" ]; then
    usage
    exit 2
fi

# build docs
cd $OPA_DIR/docs/book
gitbook install
gitbook build

# build front page
cd $OPA_DIR/docs
npm install
gulp build

# save output
tar czvf $OUTPUT_DIR/site.tar.gz -C $OPA_DIR/docs/_site/ .

if [ -n "$PORT" ]; then
    cd $OPA_DIR/docs/_site; python -m SimpleHTTPServer $PORT
fi
