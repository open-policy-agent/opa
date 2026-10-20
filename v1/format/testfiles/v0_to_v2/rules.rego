package test

import future.keywords.in

p[x] {
	x := 1
	x in input.xs
	not {x}
}

q = 1 {
	input.a
}

r { input.b }

default s = false

f(x) = y {
	y := x
}
