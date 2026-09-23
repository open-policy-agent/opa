package test

import rego.v2

p contains x if {
	some x in input.xs
	x > 1 or x < -1
}

q if every x in input.xs { x > 0 and x < 10 }

r if not { input.a; input.b }
