package test

import future.keywords.if

# A lone set term body keeps its enclosing braces, so these rules are written as
# multi-line blocks and must be separated by a blank line from the rule that
# follows them.
set_body_then_one_liner if { {1, 2} }
one_liner := 1 if input.x

set_body_then_set_body if { {1} }
another_set_body if { {2} }

# An else block is written on a line of its own.
else_rule := 1 if input.x else := 2
after_else_rule := 3 if input.x

# Rules that really are written on a single line stay grouped.
grouped_ref := 1 if input.x
grouped_negated_set := 2 if not {1}
