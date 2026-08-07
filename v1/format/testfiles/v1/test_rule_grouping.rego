package test

# Adjacent rules are only grouped without a blank line when both are written on
# a single line. A lone set term body keeps its enclosing braces, so the rules
# below are written as multi-line blocks and must be separated by a blank line.
set_body_then_one_liner if { {1, 2} }
one_liner := 1 if input.x

one_liner_then_set_body := 1 if input.x
set_body if { {1, 2} }

set_body_then_set_body if { {1} }
another_set_body if { {2} }

empty_set_body if { set() }
parenthesized_set_body if { ({1, 2}) }

partial contains 1 if { {1, 2} }
partial contains 2 if input.x

# An else block is written on a line of its own, so the rule spans more than one
# line even though its own body is written inline.
else_rule := 1 if input.x else := 2
after_else_rule := 3 if input.x

# Rules that really are written on a single line stay grouped.
grouped_ref := 1 if input.x
grouped_negated_set := 2 if not {1}
grouped_negated_empty_set := 3 if not set()
