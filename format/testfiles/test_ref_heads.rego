package test

import future.keywords

a.b.c = "d" if true
a.b.e = "f" if true
a.b.g contains x if some x in numbers.range(1, 3)
a.b.h[x] = 1 if x := "one"

q[1] = y if true
r[x] if x := 10
p.q.r[x] if x := 10
p.q.r[2] if true

g[h].i[j].k { true }
g[h].i[j].k { h := 1; j = 2 }
g[3].i[j].k = x { j := 3; x = 4 }
g[h].i[j].k[l] if { true }
g[h].i[j].k[l] contains x { x = "foo" }
g[h].i[j].k[l] contains x { h := 5; j := 6; l = 7; x = "foo" }
