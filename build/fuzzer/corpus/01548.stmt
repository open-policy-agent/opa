package z

import data.a
import input.req1
import input.req2 as req2as
import input.req3.a.b
import input.req4.a.b as req4as

p = true { a[i] = x; req1.foo = x; req2as.bar = x; q[x] }
q[x] { req1.foo = x; req2as.bar = x; r[x] }
r[x] { {"foo": req2as.bar, "bar": [x]} = {"foo": x, "bar": [req1.foo]} }
s = true { b.x[0] = 1 }
t = true { req4as.x[0] = 1 }
u[x] { b[_] = x; x > 1 }
w = [[1, 2], [3, 4]] { true }
gt1 = true { req1 > 1 }
keys[x] = y { data.numbers[_] = x; to_number(x, y) }
loopback = input { true }