#!/usr/bin/env bash
# Used by the check-changes, rego-check-pr and pr-check jobs.
set -euo pipefail

event="$1"
number="$2"
output="$3"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

gh api "repos/${GITHUB_REPOSITORY}/pulls/${number}" > "$tmp/pr.json"

gh api --paginate "repos/${GITHUB_REPOSITORY}/pulls/${number}/files" --jq '.[] | del(.patch)' \
	| jq -s '.' > "$tmp/files.json"

jq -n --arg event "$event" --slurpfile pr "$tmp/pr.json" --slurpfile files "$tmp/files.json" \
	'{event: $event, pr: $pr[0], files: $files[0]}' > "$output"
