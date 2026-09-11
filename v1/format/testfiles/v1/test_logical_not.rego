package test

import future.keywords.and
import future.keywords.not
import future.keywords.or

# `not` binds tighter than `and`/`or`
not_operand if { not input.a and input.b }

not_rhs_operand if input.a or not input.b

# a negated group is parenthesized
negated_group if { not (input.a or input.b) }

negated_group_operand if not (input.a and input.b) or input.c

# a negated body keeps its braces
negated_body if { not {input.a or input.b} }

negated_body_rhs_operand if input.a and not {input.b}

# a not-body operand keeps its braces on either side of the operator, and needs no
# parens: redundant ones are dropped
negated_body_lhs_operand if not {input.a} and input.b

negated_body_lhs_operand_parens if (not {input.a}) and input.b

negated_multi_expr_body_lhs_operand if (not {input.a; input.b}) or input.c

# a `with` on a bare operand of `not` binds to the whole `not` expression
negated_with_operand if not (input.a with input.x as 1)

# a nested negation keeps its parens: `not not x` doesn't parse
double_negation if not (not input.a)

double_negation_group if not (not (input.a or input.b))

double_negation_body if not (not {input.a; input.b})

double_negation_lhs_operand if not (not input.a) and input.b

double_negation_rhs_operand if input.a or not (not input.b)
