package pr_test

import data.pr

test_deny_long_description_for_non_maintainer_associations if {
	expected := "PR description has 1001 characters. Please shorten it to 1000 or fewer."
	some association in {"FIRST_TIMER", "FIRST_TIME_CONTRIBUTOR", "CONTRIBUTOR", "NONE"}
	expected in pr.deny with input as _pull_request(association, _repeat("a", 1001))
}

test_allow_description_at_the_limit if {
	count(pr.deny) == 0 with input as _pull_request("NONE", _repeat("a", 1000))
}

test_allow_long_description_for_maintainers if {
	some association in {"OWNER", "MEMBER", "COLLABORATOR"}
	count(pr.deny) == 0 with input as _pull_request(association, _repeat("a", 5000))
}

test_limit_counts_characters_not_bytes if {
	count(pr.deny) == 0 with input as _pull_request("CONTRIBUTOR", _repeat("é", 1000))
}

test_no_deny_when_description_is_empty if {
	count(pr.deny) == 0 with input as _pull_request("CONTRIBUTOR", null)
	count(pr.deny) == 0 with input as {"pr": {"author_association": "CONTRIBUTOR"}}
}

test_no_deny_without_pr if {
	count(pr.deny) == 0 with input as {"event": "push", "files": [{"filename": "a.go"}]}
}

_repeat(char, times) := concat("", [char | some _ in numbers.range(1, times)])

_pull_request(association, body) := {"pr": {"author_association": association, "body": body}}
