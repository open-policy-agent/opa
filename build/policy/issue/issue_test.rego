package issue_test

import data.issue

test_deny_when_type_is_not_set if {
	some input_ in [{"issue": {"type": null}}, {"issue": {}}]
	issue.deny == {"Issue is missing type (Feature, Bug, Task)"} with input as input_
}

test_deny_when_type_is_unknown if {
	input_ := {"issue": {"type": {"name": "Question"}}}
	issue.deny == {"Issue type should be one of: Feature, Bug, Task"} with input as input_
}

test_allow_valid_types[name] if {
	some name in {"Feature", "Bug", "Task"}
	count(issue.deny) == 0 with input as {"issue": {"type": {"name": name}}}
}
