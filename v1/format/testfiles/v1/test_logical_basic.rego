package test

import future.keywords.and
import future.keywords.or

both if { input.a   and   input.b }

either if { input.a or input.b }

in_body if {
	input.enabled
	input.role=="admin" or input.role=="superuser"
}

# `and` binds tighter than `or`, so no parens are needed or emitted
precedence if input.a or input.b and input.c

chained if input.a or input.b or input.c or input.d

mixed_chain if input.a and input.b or input.c and input.d

operand_terms if {
	x := input.n
	x and 1
	x or "str"
	x and [1, 2]
	x and count([1]) == 1
	count([1]) == 1 or x
}

in_comprehension if {
	xs := [x | input.a[x]; input.b[x] and input.c[x]]
	xs == [1]
}

with_every if {
	every x in [1, 2] { x > 0 }
	input.a or input.b

	{every x in [1, 2] { x > 0 }} or input.c

	input.d and {every x in [1, 2] { x > 0 }}
}

with_some if {
	some x in input.xs
	input.a[x] and input.b[x]

	{some y in input.ys;input.a[y]} and input.b

	input.c and {some z in input.zs;input.a[z]}
}

multi_line_operand_bodies if {
	{
	some v in input.vs
	input.a[v]
	} and input.b

	input.c or {
	every u in input.us { u > 0 }
	}
}
