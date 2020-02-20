#!/usr/bin/env bash
set -o errexit -o nounset -o pipefail
[[ -n "${PRE_COMMIT_VERBOSE:-}" ]] || set -o xtrace
command -v opa >/dev/null || echo "opa not found on PATH. Install it: https://github.com/open-policy-agent/opa/releases"

exit_code=0
on_error() {
    exit_code=3
}

for file_with_path in "${@}"; do
  opa fmt --write "${file_with_path}" || on_error
done

exit "${exit_code}"
