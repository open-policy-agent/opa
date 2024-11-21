# This is the same as test_issue_5449.rego, but with another rule
# that gives the formatter the assurance that using ref rules is OK
package demo

import future.keywords.contains

foo["bar"] = "baz" { input }

a.deep contains "ref"
