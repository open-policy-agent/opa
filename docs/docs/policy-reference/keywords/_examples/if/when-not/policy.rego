package play

default allow := false # not in default cases

my_constant := 42 # not for constants

# not for rule names
if {
	input.admin
}

allow if {
	# not inside rules
	input.admin if input.roles.admin == true
}

