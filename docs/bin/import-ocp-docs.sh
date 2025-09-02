#!/usr/bin/env bash

set -euo pipefail

if ! command -v curl >/dev/null 2>&1
then
  echo "curl could not be found"
  exit 1
fi

if ! command -v unzip >/dev/null 2>&1
then
  echo "unzip could not be found"
  exit 1
fi

download() {
  ref="heads/main"
  if [[ -v VERSION ]]; then
    ref="tags/$VERSION";
  fi

  url="https://github.com/open-policy-agent/opa-control-plane/archive/refs/$ref.zip"

  curl --silent -L -o ocp.zip "$url"
}

if [[ ! -e ocp.zip ]]; then
  download
else
  echo "Using existing ocp.zip"
fi

tempdir=$(mktemp -d)

unzip ocp.zip -d "$tempdir" 2>&1 > /dev/null

mv $tempdir/*/* $tempdir

ocp_docs_src="$tempdir/docs"
ocp_docs_dest="projects/ocp"

rm -rf "$ocp_docs_dest"
mkdir -p "$ocp_docs_dest"

# copy docs files
rsync --include="*.md" --exclude="*" -ah "$ocp_docs_src/." "$ocp_docs_dest/" --delete

# copy readme as index.md
index_path="$ocp_docs_dest/index.md"
cat << EOF > "$ocp_docs_dest/index.md"
---
sidebar_position: 1
---
EOF
cat "$tempdir/README.md" >> $index_path

# correct links
find "$ocp_docs_dest" -type f -name '*.md' | while read -r md_file; do
  sed -i 's|../README.md|./|g' $md_file
  sed -i 's|(./docs/|(./|g' $md_file
done
