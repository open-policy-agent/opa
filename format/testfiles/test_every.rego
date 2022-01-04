package p

import future.keywords.every
import future.keywords.in

r {
	every x in [1,3,5] {
        is_odd(x)
    }

	every x in [1,3,5] { is_odd(x); true }

	every x in [1,3,5] {
        is_odd(x)
        true
        x < 10
    }
}

is_odd(x) = x % 2 == 0