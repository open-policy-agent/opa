#!/usr/bin/env bash

set -e

git clone --quiet https://github.com/open-policy-agent/opa.git /src >/dev/null
cd /src

LAST_VERSION=$(git describe --abbrev=0 --tags)
VERSION=$1

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

update_docs() {
    find ./site/ -name "*.md" -exec sed -i='' s/${LAST_VERSION:1}/$VERSION/g {} \;
}

main() {
    update_makefile
    update_docs
    update_changelog
    git --no-pager diff --no-color
}

main
