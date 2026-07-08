package play

example_array := [1, "example", 3]

filtered_array := [e |
	some e in example_array

	is_number(e)
]
