#!/usr/bin/env bash
set -e

# Remove tests from library.
for f in $(find . -name "*_test.go"); do rm $f; done
