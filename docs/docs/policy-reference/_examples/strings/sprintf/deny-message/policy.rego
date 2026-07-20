package play

# Guests may read, but nothing else.
deny contains msg if {
	input.role == "guest"
	input.action != "read"
	msg := sprintf(
		"user %v with role %v cannot %v %v",
		[input.user, input.role, input.action, input.resource],
	)
}
