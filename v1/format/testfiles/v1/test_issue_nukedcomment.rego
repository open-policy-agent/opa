package p

rule_that_formats_ok if {
	r := other with input.x as {
		"key": {"value˚": {{
			"another": [
				[["b", "c"], "3:1:3:8"], # this
				[["b"], "4:1:4:8"], # is
				[["c"], "5:1:5:8"], # fine
			],
		}}},
	}

	r == {{}}
}

# this is a comment
# there are many like this
# but this one is mine
this_comment_above_gets_nuked_and_that_is_not_good if {
	r := set()
	r == set()
}