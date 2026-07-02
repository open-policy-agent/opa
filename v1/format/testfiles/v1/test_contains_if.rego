package test["if"]

q[x] = y if {
	y := 10
	x := "ten"
}

r contains x if { # no if here before
	x := "set"
}
