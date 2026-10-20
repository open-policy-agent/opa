package test

import future.keywords.not
import future.keywords.or

# Already a not-body; unchanged.
p if {
	x := false
	not {x}
}

q if not (input.a or input.b)
