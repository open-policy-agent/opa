#!/usr/bin/env bash
set -o errexit -o nounset -o pipefail
[[ -n "${PRE_COMMIT_VERBOSE:-}" ]] || set -o xtrace
command -v opa >/dev/null || echo "opa not found on PATH. Install it: https://github.com/open-policy-agent/opa/releases"

opa test "${@}"
