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

# function to extract title from frontmatter or first H1
extract_title() {
  local yaml_file="$1"
  local md_file="$2"

  # Try to extract title from YAML frontmatter
  local title=$(grep '^title:' "$yaml_file" | sed 's/^title: *//; s/^"//; s/"$//')

  # If no title, try sidebar_label
  if [[ -z "$title" ]]; then
    title=$(grep '^sidebar_label:' "$yaml_file" | sed 's/^sidebar_label: *//; s/^"//; s/"$//')
  fi

  # If still no title, extract from first H1 in markdown
  if [[ -z "$title" && -f "$md_file" ]]; then
    title=$(grep '^# ' "$md_file" | head -1 | sed 's/^# *//')
  fi

  # Default fallback
  if [[ -z "$title" ]]; then
    title="Regal"
  fi

  echo "$title"
}

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

if [[ -v REGAL_LOCAL_PATH && -d "$REGAL_LOCAL_PATH" ]]; then
  echo "Using local Regal directory: $REGAL_LOCAL_PATH"
  regal_docs_src="$REGAL_LOCAL_PATH/docs"
else
  if [[ ! -e regal.zip ]]; then
    download_regal
  else
    echo "Using existing regal.zip"
  fi

  tempdir=$(mktemp -d)

  unzip regal.zip -d "$tempdir" 2>&1 > /dev/null

  mv $tempdir/*/* $tempdir

  regal_docs_src="$tempdir/docs"
fi
regal_docs_dest="projects/regal"

rm -rf "$regal_docs_dest"
mkdir -p "$regal_docs_dest"

# copy assets
rsync -ah "$regal_docs_src/assets/." "$regal_docs_dest/assets" --delete

# generate index from readme-sections
readme_sections_dir="$regal_docs_src/readme-sections"
manifest="$readme_sections_dir/website-manifest"

{
  while IFS= read -r file; do
    section_path="$readme_sections_dir/$file"
    if [[ -f "$section_path" ]]; then
      cat "$section_path"
      echo ""
    fi
  done < "$manifest"
} > "$regal_docs_dest/index.md"

# copy rules directory
cp -r "$regal_docs_src/rules" "$regal_docs_dest/"

# process .md.yaml pairs to add head metadata
find "$regal_docs_src" -type f -name '*.md.yaml' | while read -r yaml_file; do
  md_file="$(dirname $yaml_file)/$(basename "$yaml_file" .yaml)"
  md_file_rel=${md_file#"$regal_docs_src/"}
  dest_md_file="$regal_docs_dest/$md_file_rel"

  mkdir -p "$(dirname $dest_md_file)"

  if [[ ! -e $md_file ]]; then
    echo "Warning: $md_file missing"
  else
    # extract title for head metadata
    title=$(extract_title "$yaml_file" "$md_file")

    # generate file with frontmatter, head metadata, and content
    {
      echo -e "---\n$(cat $yaml_file)\n---\n"
      echo -e "<head>\n  <title>$title | Regal</title>\n</head>\n"
      cat $md_file
    } > "$dest_md_file"
  fi
done
