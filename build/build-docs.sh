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

BOOK_DIR=$OPA_DIR/docs/book
pushd $BOOK_DIR
gitbook -v 3.2.3 build
popd

SITE_DIR=$OPA_DIR/docs/_site
rm -fr $SITE_DIR && mkdir $SITE_DIR
cp -r $BOOK_DIR/_book/ $SITE_DIR/docs
cp $OPA_DIR/docs/index.html $SITE_DIR/
cp $OPA_DIR/docs/style.css $SITE_DIR/
cp -r $OPA_DIR/docs/img $SITE_DIR/
tar czvf $OUTPUT_DIR/site.tar.gz -C $SITE_DIR .

if [ -n "$PORT" ]; then
    cd $SITE_DIR; python -m SimpleHTTPServer $PORT
fi
