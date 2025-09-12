package play

import rego.v1

default max_amount(_, _) := 1000

max_amount(overrides, role) := overrides[role]

allow if {
	input.amount <= max_amount(data.overrides, input.role)
}
