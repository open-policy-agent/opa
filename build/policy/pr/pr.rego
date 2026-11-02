# METADATA
# description: |
#   Expects policy input as provided by build/gen-checks-input.sh. The "pr" key
#   has the response of:
#   https://api.github.com/repos/open-policy-agent/opa/pulls/${PR_ID}
#
#   Runs without a "pr" key, like on a push to main, do nothing.
package pr

max_description_length := 1000

# Maintainers can write what they need. These authors are new to the project.
non_maintainer_associations := {"FIRST_TIMER", "FIRST_TIME_CONTRIBUTOR", "CONTRIBUTOR", "NONE"}

# METADATA
# entrypoint: true
deny contains $"PR description has {length} characters. Please shorten it to {max_description_length} or fewer." if {
	input.pr.author_association in non_maintainer_associations
	not _is_dependabot

	length := count(input.pr.body)
	length > max_description_length
}

_is_dependabot if input.pr.user.login == "dependabot[bot]"
