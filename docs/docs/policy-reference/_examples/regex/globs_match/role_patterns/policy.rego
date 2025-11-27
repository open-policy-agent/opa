package play

import rego.v1

user_roles := data.user_roles[input.user_id]

action_requirements := data.action_requirements[input.action]

permission_patterns contains pattern if {
	some role in user_roles
	some pattern in data.role_permissions[role]
}

default allow := false

allow if {
	every requirement in action_requirements {
		some pattern in permission_patterns
		regex.globs_match(pattern, requirement)
	}
}
