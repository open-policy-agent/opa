package play

import rego.v1

default is_sudo(_) := false

is_sudo(user) if {
	user.role == "admin"
}

is_sudo(user) if {
	user.sudo == true
}

allow if is_sudo(input.user)
