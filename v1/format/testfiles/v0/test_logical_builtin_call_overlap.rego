package test

import future.keywords.and
import future.keywords.or

p = or({1}, {2})

q = and({1, 2}, {2, 3})

r {
	or({1}, {2}) == {1, 2}
}

s = x {
	x = and(input.a, input.b)
	x == or(input.c, input.d)
}

t = or(count(input.a), 1)
