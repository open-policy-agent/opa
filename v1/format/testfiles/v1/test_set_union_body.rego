package test

# A `|` leading the first expression of a `p if { ... }` body would make the
# braces read as a set comprehension, so the union keeps its parens there.
lone if {
	(input.a | input.b)
}

leading if {
	(input.a | input.b)
	input.c
}

under_comparison if {
	(input.a | input.b) == input.c
}

under_unification if {
	(input.a | input.b) = input.c
}

with_modifier if {
	(input.a | input.b) with input.x as 1
}

partial contains 1 if {
	(input.a | input.b)
}

function(_) if {
	(input.a | input.b)
}

value := 1 if {
	(input.a | input.b)
}

# No ambiguity when the union doesn't lead the braces
trailing if {
	input.c
	(input.a | input.b)
}

assigned if {
	x := (input.a | input.b)
	x
}

negated if {
	not (input.a | input.b)
}

rhs_operand if {
	input.c == (input.a | input.b)
}

nested_call if {
	(input.a | input.b) & input.c == input.d
}

# An `else` body, an `every` body and a comprehension body are never read as a term
else_body if {
	input.z
} else if {
	(input.a | input.b)
}

every_body if {
	every x in input.xs {
		(input.a | input.b)
		x
	}
}

comprehension_body := {y |
	(input.a | input.b)
	y := 1
}

# Sanity: comprehension syntax is untouched
set_comprehension := {x | some x in input.xs}

object_comprehension := {k: v | some k, v in input.o}

array_comprehension := [x | some x in input.xs]

union_in_comprehension_head := {(input.a | input.b) | input.c}

comprehension_term_body if {
	{x | input.a[x]}
}

comprehension_term_body_oneline if {x | input.a[x]}

comprehension_in_expression if {
	{x | input.a[x]} == input.b
}

comprehension_statement if {
	s := {x | some x in input.xs}
	count(s) > 0
}
