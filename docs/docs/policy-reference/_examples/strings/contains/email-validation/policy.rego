package play

import rego.v1

example1 if contains("alice@example.com", "@")

example2 if contains("bob[at]example.com", "@")

example3 if contains(input.email, "@")
