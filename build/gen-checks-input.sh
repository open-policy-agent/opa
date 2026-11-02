#!/usr/bin/env bash
# Writes the input document that the policies in build/policy read. It has the
# event name, the pull request and its changed files. Every CI check that runs
# one of those policies uses this same document.
#
# Usage: gen-checks-input.sh <event name> <pull request number> <output file>
# Needs GH_TOKEN and GITHUB_REPOSITORY to be set, as they are in GitHub Actions.
set -euo pipefail

event="$1"
number="$2"
output="$3"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

gh api "repos/${GITHUB_REPOSITORY}/pulls/${number}" > "$tmp/pr.json"

# The files endpoint returns one page at a time, so ask for all of them. The
# patch text is large and no policy needs it.
gh api --paginate "repos/${GITHUB_REPOSITORY}/pulls/${number}/files" --jq '.[] | del(.patch)' \
	| jq -s '.' > "$tmp/files.json"

jq -n --arg event "$event" --slurpfile pr "$tmp/pr.json" --slurpfile files "$tmp/files.json" \
	'{event: $event, pr: $pr[0], files: $files[0]}' > "$output"
