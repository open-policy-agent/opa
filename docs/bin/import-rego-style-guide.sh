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

download_style_guide() {
  # Always download from main branch
  ref="heads/main"

  # https://github.com/open-policy-agent/rego-style-guide/archive/refs/heads/main.zip
  url="https://github.com/open-policy-agent/rego-style-guide/archive/refs/$ref.zip"

  curl --silent -L -o rego-style-guide.zip "$url"
}

if [[ ! -e rego-style-guide.zip ]]; then
  download_style_guide
else
  echo "Using existing rego-style-guide.zip"
fi

tempdir=$(mktemp -d)

unzip rego-style-guide.zip -d "$tempdir" 2>&1 > /dev/null

mv $tempdir/*/* $tempdir

style_guide_src="$tempdir/style-guide.md"

# Destination path relative to docs directory
style_guide_dest="docs/style-guide.md"

# Copy the markdown file
if [[ -f "$style_guide_src" ]]; then
  cp "$style_guide_src" "$style_guide_dest"
  echo "Copied style-guide.md to $style_guide_dest"
else
  echo "Error: style-guide.md not found in $tempdir"
  exit 1
fi

# Clean up
rm -rf "$tempdir"

echo "Style guide import complete!"
