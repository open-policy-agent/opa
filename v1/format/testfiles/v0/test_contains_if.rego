package test.if

import future.keywords

q[x] = y if {
	y := 10
	x := "ten"
}

q[x] = y { # not using if
	y := 11
	x := "eleven"
}

r[x] { # no if here before
	x := "set"
}
