package test

keys := ["a", "b"]

# a parenthesized value keeps its parentheses: without them the comprehension
# bar reads as a set union and the formatted output no longer parses
p {
	x := {k: (1 + 2) | k := keys[_]}
	x
}

# the same value, written on the row below the key
q {
	x := {
		k: (1 + 2) | k := keys[_]
	}
	x
}
