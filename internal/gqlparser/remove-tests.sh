#!/usr/bin/env bash
set -e

# Remove tests from library.
find . -name "*_test.go" -delete
