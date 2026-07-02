package test

import future.keywords.if

a(_)

b("foo")

c(x) {
	x == 1
}

d(x) if {
	x == 1
}

e(x) := x

f(x) := x {
	x == 1
}

g(x) := x if {
	x == 1
}

h.i(_)

i.j("foo")

j.k(x) {
	x == 1
}

k.l(x) if {
	x == 1
}

l.m(x) := x

m.n(x) := x {
	x == 1
}

n.o(x) := x if {
	x == 1
}

o.p.q(_)

p.q.r("foo")

q.r.s(x) {
	x == 1
}

r.s.t(x) if {
	x == 1
}

s.t.u(x) := x

t.u.v(x) := x {
	x == 1
}

u.v.w(x) := x if {
	x == 1
}
