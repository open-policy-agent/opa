package test

test_x if {
	r := allow with input as {
		"a": "b", # a comment
	}
	with opa.runtime as {"env": {"E": "PROD"}}
	r == false
}
