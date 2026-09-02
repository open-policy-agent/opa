package test

import future.keywords.and
import future.keywords.or

# Line breaks are preserved; continuation lines get one extra indent level
broken_chain if {
	input.a and
	input.b and
	input.c
}

over_indented if {
	input.a and
			input.b and
	input.c
}

mixed_operators if {
	input.a and input.b or
	input.c and input.d
}

partly_broken if {
	input.a and input.b and
	input.c
}

nested_group_breaks if {
	input.a and (input.b or
	input.c)
}

broken_group_lhs if {
	(input.a or
	input.b) and input.c
}

already_formatted if {
	input.a and
		input.b or
		input.c
}

# An explicit `{...}` operand on a line of its own stays there
broken_body_chain if {
	{input.foo.bar.baz.a} or
	{input.foo.bar.baz.b} or
	{input.foo.bar.baz.c} or
	{input.foo.bar.baz.d} or
	{input.foo.bar.baz.e} or
	{input.foo.bar.baz.f} or
	{input.foo.bar.baz.g}
}

broken_mixed_operands if {
	{input.a} or
	input.b or
	{input.c}
}

broken_multi_expr_body_rhs if {
	{input.a} or
	{
		input.b
		input.c
	}
}

broken_after_multi_expr_body_lhs if {
	{
		input.a
		input.b
	} or
	{input.c}
}

# A brace opening on the operator's line is not a break
body_opens_on_operator_line if {
	{input.a} or {
		input.b
	} or {input.c}
}
