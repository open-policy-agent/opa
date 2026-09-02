package test

import future.keywords.and
import future.keywords.not
import future.keywords.or

deep_chain if { {a := input.a; a > 0} and (input.b or {input.c and input.d}) }

nested_in_comprehension if {
	xs := [x | input.a[x]; input.b[x] and (input.c[x] or input.d[x])]
	ys := {x | input.a[x]; input.b[x] or {input.c[x]; input.d[x]}}
	count(xs) == count(ys)
}

nested_in_every if {
	every x in input.xs {
		x.a and (x.b or x.c)
	}
}

nested_in_not if not (input.a and (input.b or not input.c))

with_else := 1 if {input.a} and input.b else := 2

nested_body_in_body if {
	{
		input.a and {input.b
		input.c }
	} or {
		input.d
		input.e
	}
}
