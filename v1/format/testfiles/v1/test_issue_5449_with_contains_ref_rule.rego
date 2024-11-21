# This is the same as test_issue_5449.rego, but with another rule
# that gives the formatter the assurance that using ref rules is OK
package demo

foo["bar"] = "baz" if { input }

a.deep contains "ref"
