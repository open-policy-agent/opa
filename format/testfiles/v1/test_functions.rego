package p

f1(x) = x

f2(x) := x

f3(1)

f4(1) if {
	true
}

f5(1) := x if {
	x := 5
}

f6(x) if {
	true
} else := false

f7(x) := 1 if {
	x.key1
} else := false if {
	x.key2
} else if {
	false
}

f(x) = 1 if {
	input.x == x
} {
	input.x < 10
}

f(_) if {
	input.x
} {
	input.y
}

# Non-functions
foo[x] = y if {
	x = 1
	y = 2
} {
	x = 3
	y = 4
}
