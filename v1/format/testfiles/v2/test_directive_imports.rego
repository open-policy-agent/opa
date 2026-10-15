package test

# Directive imports are no-ops in v2, and kept unless dropping redundant directive imports is requested.
import rego.v1
import rego.v2
import future.keywords
import future.keywords.in
import data.x

p if 1 in x
