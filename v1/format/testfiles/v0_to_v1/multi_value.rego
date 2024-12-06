package test

import future.keywords.contains
import future.keywords.if

a.b

b.c {
	input.x
}

c contains "d"

d contains "e" if {
	input.x
}

e.f contains "g" {
	input.x
}

f.g contains "h" if {
	input.x
}

g[h].i contains "j" {
	h := input.h
}

h[i].j contains "k" if {
	i := input.h
}
