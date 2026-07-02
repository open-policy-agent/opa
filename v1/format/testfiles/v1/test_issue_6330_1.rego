package a

# A ref-head rule by itself is formatted differently when there are multiple ref-head rules in a file.
# This file handles the case when there is a single ref-head rule, while `test_issue_6330.rego` handles multiple.

p[
{"a": #
"b"} #
] := true