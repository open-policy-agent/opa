#!/usr/bin/env bash
set -eo pipefail
OPA_EXEC="$1"
TARGET="$2"

PATH_SEPARATOR="/"
if [[ $OPA_EXEC == *".exe" ]]; then
    PATH_SEPARATOR="\\"
fi



github_actions_group() {
    local args="$*"
    echo "::group::$args"
    $args
    echo "::endgroup::"
}

opa() {
    local args="$*"
    github_actions_group $OPA_EXEC $args
}

# assert_contains checks if the actual string contains the expected string.
assert_contains() {
    local expected="$1"
    local actual="$2"
    if [[ "$actual" != *"$expected"* ]]; then
        echo "Expected '$expected' but got '$actual'"
        exit 1
    fi
}




opa version
opa eval -t $TARGET 'time.now_ns()'
opa eval --format pretty --bundle test/cli/smoke/golden-bundle.tar.gz --input test/cli/smoke/input.json data.test.result --fail
opa exec --bundle test/cli/smoke/golden-bundle.tar.gz --decision test/result test/cli/smoke/input.json
opa build --output o0.tar.gz test/cli/smoke/data.yaml test/cli/smoke/test.rego
echo '{"yay": "bar"}' | opa eval --format pretty --bundle o0.tar.gz -I data.test.result --fail
opa build --optimize 1 --output o1.tar.gz test/cli/smoke/data.yaml test/cli/smoke/test.rego
echo '{"yay": "bar"}' | opa eval --format pretty --bundle o1.tar.gz -I data.test.result --fail
opa build --optimize 2 --output o2.tar.gz  test/cli/smoke/data.yaml test/cli/smoke/test.rego
echo '{"yay": "bar"}' | opa eval --format pretty --bundle o2.tar.gz -I data.test.result --fail

# Tar paths 
opa build --output o3.tar.gz test/cli/smoke
github_actions_group assert_contains '/test/cli/smoke/test.rego' "$(tar -tf o3.tar.gz /test/cli/smoke/test.rego)"

# Data files - correct namespaces
echo "::group:: Data files - correct namespaces"
assert_contains "data.namesapce | test${PATH_SEPARATOR}cli${PATH_SEPARATOR}smoke${PATH_SEPARATOR}namesapce${PATH_SEPARATOR}data.json" "$(opa inspect test/cli/smoke)"
echo "::endgroup::"