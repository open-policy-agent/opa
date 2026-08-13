package test

import future.keywords.and
import future.keywords.or

redundant if { ((input.a or input.b)) }

right_nested_or_keeps_parens if input.a or (input.b or input.c)

left_nested_or_drops_parens if (input.a or input.b) or input.c

right_nested_and_keeps_parens if input.a and (input.b and input.c)

left_nested_and_drops_parens if (input.a and input.b) and input.c

and_under_or_drops_parens if (input.a and input.b) or input.c

and_under_or_drops_parens_rhs if input.c or (input.a and input.b)

or_under_and_keeps_parens if input.c and (input.a or input.b)

or_under_and_keeps_parens_lhs if (input.a or input.b) and input.c

deeply_nested if input.a or (input.b and (input.c or input.d))

# A brace-led operand would be read back as an explicit body, so it keeps its parens
set_term_lhs if ({input.a}) or input.b

set_term_rhs if input.b or ({input.a})

object_term_lhs if ({"b": 1}) or input.a

object_term_rhs if input.a or ({"b": 1})

comprehension_term_lhs if ({x | input.a[x]}) or input.b

comprehension_term_rhs if input.b or ({x | input.a[x]})

comparison_with_set_term_lhs if ({input.a} == input.b) and input.c

comparison_with_set_term_rhs if input.c and ({input.a} == input.b)

ref_into_set_term_lhs if ({input.a}[0]) and input.b

ref_into_set_term_rhs if input.b and ({input.a}[0])

# a brace-led operand nested inside an infix call still leads the rendering
nested_brace_lead_lhs if ({1, 2}) & input.s == set() and input.a

nested_brace_lead_rhs if input.a and ({1, 2}) & input.s == set()

nested_brace_lead_ref_lhs if ({"b": 1}.b) == 1 and input.a

nested_brace_lead_ref_rhs if input.a and ({"b": 1}.b) == 1

# `x | y` is a set union here, and needs no parens outside of braces
set_union_lhs if (input.a | input.b) or input.c

set_union_rhs if input.c or (input.a | input.b)

# inside braces it would read as a comprehension, so it keeps its parens
set_union_in_body_lhs if {(input.a | input.b)} or input.c

set_union_in_body_rhs if input.c or {(input.a | input.b)}

# in a rule body the leading `|` would make the braces read as a comprehension
set_union_in_rule_body if {
	(input.a | input.b) or input.c
}

set_union_in_rule_body_nested if {
	((input.a | input.b) == input.c) or input.d
}

set_union_in_rule_body_with if {
	(input.a | input.b) or input.c with input.x as 1
}

set_union_operand_with_body if input.c or {(input.a | input.b) with input.x as 1}

set_union_operand_with_parens if input.c or ((input.a | input.b) with input.x as 1)
