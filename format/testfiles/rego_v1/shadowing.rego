package test

input := "foo"

input.x := "bar"

data := "baz"

data.x := "bax"

p {
	input := 1
	data := 2
}

q[input] {
	input := 3
}

r[data] {
	data := 4
}

s(input) {
	input == 5
}

t(data) {
	data == 6
}
