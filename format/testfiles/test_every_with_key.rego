package p

import future.keywords.every
import future.keywords.in

r {
	every i, x in [1,3,5] {
        is_odd(x)
        i < 10
    }

	every i, x in {"foo": 1, "bar": 3, "baz": 5} { is_odd(x); i != 20 }

	every i, x in [1,3,5] {
        is_odd(x)
        true
        x < 10
        x > i
    }
}

is_odd(x) = x % 2 == 0