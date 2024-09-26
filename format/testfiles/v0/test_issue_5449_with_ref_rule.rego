# This is the same as test_issue_5449.rego, but with a rule that gives
# the formatter the assurance that using ref rules is OK
package demo

foo["bar"] = "baz" { input }

a.deep.ref := true
