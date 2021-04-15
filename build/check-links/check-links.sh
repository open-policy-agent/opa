#!/bin/bash

set -eu

FAILED=0

Help()
{
   # Display Help
   echo "Parsing all files in the repo"
   echo
   echo "Syntax: check-links.sh [-v]"
   echo "options:"
   echo "h     Print this Help."
   echo "v     Verbose mode."
   echo
}

while getopts ":h" option; do
   case $option in
      h) # display Help
         Help
         exit;;
   esac
done


SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
REPO_DIR="$( cd "${SCRIPT_DIR}/../../" && pwd )"
for MD in $(find "${REPO_DIR}" -path "${REPO_DIR}"/vendor -prune -o -name "*.md" | sort)
do
  python3 "${SCRIPT_DIR}"/check-links.py --file "$MD" "$@" || (( FAILED += $? ))
done

exit ${FAILED}