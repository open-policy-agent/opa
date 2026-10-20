package test

import rego.v1
import rego.v2
import future.keywords.in
import future.keywords.and

p if input.a and input.b

q if 1 in input.xs
