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

test_allow_long_description_for_dependabot if {
	dependabot := {"pr": {"user": {"login": "dependabot[bot]", "type": "Bot"}}}
	input_ := object.union(_pull_request("CONTRIBUTOR", _repeat("a", 5000)), dependabot)
	count(pr.deny) == 0 with input as input_
}

test_limit_counts_characters_not_bytes if {
	count(pr.deny) == 0 with input as _pull_request("CONTRIBUTOR", _repeat("é", 1000))
}

test_no_deny_when_description_is_empty if {
	count(pr.deny) == 0 with input as _pull_request("CONTRIBUTOR", null)
	count(pr.deny) == 0 with input as {"pr": {"author_association": "CONTRIBUTOR"}}
}

test_title_prefix[note] if {
	some note, tc in {
		"area prefix": {
			"title": "ast: Fix a bug",
			"exp": set(),
		},
		"multiple areas": {
			"title": "ast, topdown: Fix a bug",
			"exp": set(),
		},
		"release": {
			"title": "Release 1.2.3",
			"exp": set(),
		},
		"no prefix": {
			"title": "Fix a bug",
			"exp": {"PR titles start with pkg/area/topic, (e.g. ast: Fix a bug)"},
		},
		"no space after comma": {
			"title": "ast,topdown: Fix a bug",
			"exp": {"PR titles start with pkg/area/topic, (e.g. ast: Fix a bug)"},
		},
	}

	pr.deny == tc.exp with input as {"pr": {"author_association": "OWNER", "title": tc.title}}
}

test_no_deny_without_pr if {
	count(pr.deny) == 0 with input as {"event": "push", "files": [{"filename": "a.go"}]}
}

_repeat(char, times) := concat("", [char | some _ in numbers.range(1, times)])

_pull_request(association, body) := {"pr": {"author_association": association, "body": body}}
