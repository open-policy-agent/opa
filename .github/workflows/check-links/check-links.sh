#!/bin/bash

set -eu




FAILED=0

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
REPO_DIR="$( cd "${SCRIPT_DIR}/../../../" && pwd )"
for MD in $(find "${REPO_DIR}" -path "${REPO_DIR}"/vendor -prune -o -name "*.md" | sort)
do
  python3 "${SCRIPT_DIR}"/check-links.py $MD || (( FAILED += $? ))
done

exit ${FAILED}