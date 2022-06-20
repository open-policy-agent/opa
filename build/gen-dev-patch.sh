#!/usr/bin/env bash

set -e

OPA_DIR=/go/src/github.com/open-policy-agent/opa

usage() {
    echo "gen-dev-patch.sh --source-url=<git-url>"
    echo "                 --version=<mj.mn.pt>"
}

for i in "$@"; do
    case $i in
    --source-url=*)
        SOURCE_URL="${i#*=}"
        shift
        ;;
    --version=*)
        VERSION="${i#*=}"
        shift
        ;;
    *)
        usage
        exit 1
        ;;
    esac
done

if [ -z "$SOURCE_URL" ]; then
    usage
    exit 1
elif [ -z "$VERSION" ]; then
    usage
    exit 1
fi

git clone $SOURCE_URL $OPA_DIR
cd $OPA_DIR

LAST_VERSION=$(git describe --abbrev=0 --tags | cut -c 2-)

update_version() {
    ./build/update-version.sh "$VERSION-dev"
}

update_changelog() {
    cat >_CHANGELOG.md <<EOF
$(awk "1;/## $LAST_VERSION/{exit}" CHANGELOG.md | sed '$d')

## Unreleased

## $LAST_VERSION
$(sed "1,/## $LAST_VERSION/d" CHANGELOG.md)
EOF

    mv _CHANGELOG.md CHANGELOG.md
}

main() {
    update_version
    update_changelog
    git --no-pager diff --no-color
}

main
