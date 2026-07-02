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

download_cheatsheet() {
  # Always download from main branch
  ref="heads/main"

  # https://github.com/open-policy-agent/rego-cheat-sheet/archive/refs/heads/main.zip
  url="https://github.com/open-policy-agent/rego-cheat-sheet/archive/refs/$ref.zip"

  curl --silent -L -o rego-cheat-sheet.zip "$url"
}

if [[ ! -e rego-cheat-sheet.zip ]]; then
  download_cheatsheet
else
  echo "Using existing rego-cheat-sheet.zip"
fi

tempdir=$(mktemp -d)

unzip rego-cheat-sheet.zip -d "$tempdir" 2>&1 > /dev/null

mv $tempdir/*/* $tempdir

cheatsheet_src="$tempdir/build"

# Destination paths relative to docs directory
cheatsheet_md_dest="docs/cheatsheet.md"
cheatsheet_pdf_dest="static/cheatsheet.pdf"

# Copy the markdown file
if [[ -f "$cheatsheet_src/cheatsheet.md" ]]; then
  cp "$cheatsheet_src/cheatsheet.md" "$cheatsheet_md_dest"
  echo "Copied cheatsheet.md to $cheatsheet_md_dest"
else
  echo "Error: cheatsheet.md not found in $cheatsheet_src"
  exit 1
fi

# Copy the PDF file
if [[ -f "$cheatsheet_src/cheatsheet.pdf" ]]; then
  cp "$cheatsheet_src/cheatsheet.pdf" "$cheatsheet_pdf_dest"
  echo "Copied cheatsheet.pdf to $cheatsheet_pdf_dest"
else
  echo "Error: cheatsheet.pdf not found in $cheatsheet_src"
  exit 1
fi

# Clean up
rm -rf "$tempdir"

echo "Cheat sheet import complete!"
