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
