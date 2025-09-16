package play

default allow := false

allow if {
	every email, user in input.invites {
		endswith(email, "@example.com")
		"staff" in user.roles
	}
}
