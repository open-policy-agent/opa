package test

a.b := "c" {
	input.x
}

b["c/d"] := "e" {
	input.d
}

c.d.e

d.e.f {
	input.x
}

e[f] := "g" {
	f := input.f
}

f["g"] := "h" {
	input.x
}

g.h[i].j[k] := "l" {
	i := input.i
	k := input.k
}

h.i["j/k"].l["m"] := "n" {
	input.x
}
