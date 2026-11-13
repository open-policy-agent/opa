# METADATA
# description: |
#   Expects policy input with an "issue" key. It has the response of:
#   https://api.github.com/repos/open-policy-agent/opa/issues/${ISSUE_ID}
package issue

# METADATA
# entrypoint: true
deny contains "Issue is missing type (Feature, Bug, Task)" if {
	not input.issue.type.name
}

# This cannot happen today. It is here to catch issues if types become configurable.
deny contains "Issue type should be one of: Feature, Bug, Task" if {
	not input.issue.type.name in {"Feature", "Bug", "Task"}
}
