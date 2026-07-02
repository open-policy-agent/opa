package test

test_with_and_indentation if {
	my_mock_object := {
		"the_first_key": {
			"a_nested_key": {
				"and_sub_nested_key": [
					{
						"yet_another_key": [
							"and_sub_nested_key_value", "foooo", "baaar",
						],
					},
				],
			}
		}
	}

	_ = my_mock_object
}
