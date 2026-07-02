package play

import rego.v1

default allow := false

allow if count(reasons) == 0

reasons contains "name must be set" if {
	object.get(input, "name", "") == ""
}

reasons contains "@example.com email blocked" if {
	endswith(input.email, "@example.com")
}

reasons contains message if {
	input.age < 18

	message := sprintf(
		"you must be %d year older",
		[18 - input.age],
	)
}
