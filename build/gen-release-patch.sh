#!/usr/bin/env bash

set -e

OPA_DIR=/go/src/github.com/open-policy-agent/opa

usage() {
    echo "gen-release-patch.sh --source-url=<git-url>"
    echo "                     --version=<mj.mn.pt>"
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

if [ -z "$LAST_VERSION" ]; then
    LAST_VERSION=$(git describe --abbrev=0 --tags)
fi

update_makefile() {
    sed -i='' -e "s/^VERSION[ \t]*:=[ \t]*.\+$/VERSION := $VERSION/" Makefile
}

update_changelog() {
    cat >_CHANGELOG.md <<EOF
$(awk '1;/## Unreleased/{exit}' CHANGELOG.md | sed '$d')

## $VERSION

$(./build/changelog.py $LAST_VERSION HEAD)
$(sed '1,/## Unreleased/d' CHANGELOG.md)
EOF

    mv _CHANGELOG.md CHANGELOG.md
}

main() {
    update_makefile
    update_changelog
    git --no-pager diff --no-color
}

main
