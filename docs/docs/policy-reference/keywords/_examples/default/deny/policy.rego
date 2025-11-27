package play

import rego.v1

default allow := false

allow if input.admin == true

allow if {
	input.path[0] == "users"
	input.path[1] == input.user
}
