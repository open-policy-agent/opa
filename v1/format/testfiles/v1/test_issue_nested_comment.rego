package p

fmt_bug := {
	"a": [{
		"b": 1,
		# this comment seems to trigger the issue
		"c": 2,
	}],
	"d": 3,
}

default this_should_not_get_indented := {}

this_should_not_get_indented := input.x

fmt_bug := {
	"a": [{
		"a": [{
    		"b": 1,
    		# this comment seems to trigger the issue
    		"c": 2,
    	}],
		"b": 1,
		# this comment seems to trigger the issue
		"c": 2,
	}],
	"d": 3,
}

default this_should_not_get_indented := {}

this_should_not_get_indented := input.x