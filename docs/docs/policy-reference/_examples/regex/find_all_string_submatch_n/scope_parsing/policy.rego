package play

import rego.v1

scope_pattern := `(\w+):(\w+)`

scope_map[scope[2]] := scope[1] if {
	some scope in regex.find_all_string_submatch_n(scope_pattern, input.token.payload.scopes, -1)
}

resource := split(input.path, "/")[1]

default allow := false

allow if {
	input.method == "GET"
	scope_map[resource] in {"read", "write"}
}
