package test

import future.keywords.not

a if {
	not input.x + input.y == 1
	not {input.x + input.y == 2}
	not { input.x + input.y == 3 }
	not {
		input.x + input.y == 4
	}

	not {input.a == 1;input.b == 2}
	not {
		input.a == 1
		input.b == 2
	}

	# comment 1
	not { # comment 2
		# comment 3
		input.a == 1
		input.b == 2 # comment 4
		# comment 5
	} # comment 6
	# comment 7
}

b if {
	not { f(42) with input.x as 1 } with input.y as 2

	not {
		f(42) with input.x as 1 with input.x2 as 2
			with input.x3 as 3
	with input.x4 as 4
	} with input.y as 5 with input.y2 as 6
		with input.y3 as 7
	with input.y4 as 8
}