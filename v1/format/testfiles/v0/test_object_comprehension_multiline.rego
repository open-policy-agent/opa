package test

objects := {"a": 1, "b": 2}

keys := ["a", "b"]

# the key sits on the row directly below the opening brace
p {
	x := {
		k: objects[k] | k := keys[_]
	}
	x
}

# the key sits more than one row below the opening brace
q {
	x := {

		k: objects[k] | k := keys[_]
	}
	x
}

# already on one line
r {
	x := {k: objects[k] | k := keys[_]}
	x
}
