package test.if2 # FIXME: refs aren't allowed to contains keywords

q[x] = y if {
	y := 10
	x := "ten"
}

r contains x if { # no if here before
	x := "set"
}
