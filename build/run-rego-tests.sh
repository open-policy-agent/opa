#!/usr/bin/env bash
# Runs `opa test` over every directory that contains *_test.rego files, so Rego
# unit tests for embedded/internal policies are exercised in CI. build/policy is
# excluded because it is tested separately with its JSON schemas; test fixtures
# are excluded because they are not meant to be run as tests.
set -euo pipefail

OPA="${OPA:-opa}"

dirs=$(find . -name '*_test.rego' \
	-not -path './build/policy/*' \
	-not -path '*/testdata/*' \
	-not -path '*/testfiles/*' \
	-not -path '*/node_modules/*' \
	-not -path './.git/*' \
	-not -path './.claude/*' \
	-exec dirname {} \; | sort -u)

if [ -z "$dirs" ]; then
	echo "No Rego test directories found."
	exit 0
fi

status=0
for d in $dirs; do
	echo "==> ${OPA} test ${d}"
	"$OPA" test "$d" || status=1
done

exit "$status"
