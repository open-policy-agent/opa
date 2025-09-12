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

download_regal() {
  ref="heads/main"
  if [[ -v VERSION ]]; then
    ref="tags/$VERSION";
  fi

  # examples
  # https://github.com/open-policy-agent/regal/archive/refs/heads/main.zip
  # https://github.com/open-policy-agent/regal/archive/refs/tags/v0.35.1.zip
  url="https://github.com/open-policy-agent/regal/archive/refs/$ref.zip"

  curl --silent -L -o regal.zip "$url"
}

if [[ ! -e regal.zip ]]; then
  download_regal
else
  echo "Using existing regal.zip"
fi

tempdir=$(mktemp -d)

unzip regal.zip -d "$tempdir" 2>&1 > /dev/null

mv $tempdir/*/* $tempdir

regal_docs_src="$tempdir/docs"
regal_docs_dest="projects/regal"

rm -rf "$regal_docs_dest"
mkdir -p "$regal_docs_dest"

# copy assets
rsync -ah "$regal_docs_src/assets/." "$regal_docs_dest/assets" --delete

# generate index
readme_sections_dir="$regal_docs_src/readme-sections"
manifest="$readme_sections_dir/website-manifest"

tmpfile=$(mktemp)

while IFS= read -r file; do
  section_path="$readme_sections_dir/$file"

  if [[ -f "$section_path" ]]; then
    cat "$section_path" >> "$tmpfile"
    echo -e "\n" >> "$tmpfile"
  else
    echo "Section file not found: $section_path" >&2
    exit 1
  fi
done < "$manifest"

mv $tmpfile "$regal_docs_dest/index.md"

# copy in rules
cp -r "$regal_docs_src/rules" "$regal_docs_dest/"

# generate other files
find "$regal_docs_src" -type f -name '*.md.yaml' | while read -r yaml_file; do
  md_file="$(dirname $yaml_file)/$(basename "$yaml_file" .yaml)"
  md_file_rel=${md_file#"$regal_docs_src/"}
  dest_md_file="$regal_docs_dest/$md_file_rel"

  mkdir -p "$(dirname $dest_md_file)"

  if [[ ! -e $md_file ]]; then
    echo "Warning: $md_file missing"
  else
    echo -e "---\n$(cat $yaml_file)\n---\n\n" > $dest_md_file
    cat $md_file >> $dest_md_file
  fi
done
