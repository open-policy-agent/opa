package test

input := "do"

input.x := "re"

input.x.y := "mi"

data := "fa"

data.x := "so"

data.x.y := "la"

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
