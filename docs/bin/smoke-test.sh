#!/usr/bin/env bash

# this script contains a number of automated tests for website URLs that are
# depended on externally. During the rollout of the new website, we hit some
# issues with these URLs not being available so this script is here to be run
# in post merge to ensure we don't break them again.

urls=(
  "https://www.openpolicyagent.org/downloads/v1.4.2/opa_darwin_arm64_static"
  "https://www.openpolicyagent.org/bundles/helm-kubernetes-quickstart"
  "https://www.openpolicyagent.org/img/logos/opa-horizontal-color.png"
  "https://www.openpolicyagent.org/img/logos/opa-no-text-color.png" 
)

exit_code=0

for url in "${urls[@]}"; do
  echo -n "Testing $url ... "

  status=$(curl -s -o /dev/null -w "%{http_code}" -I "$url")

  if [[ "$status" =~ ^2|^3 ]]; then
    echo "PASS ($status)"
  else
    echo "FAIL ($status)"
    exit_code=1
  fi
done

exit $exit_code
