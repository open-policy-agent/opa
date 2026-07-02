package p

# existing future.keywords imports kept for broadest compatibility surface
import future.keywords.every
import future.keywords.if

r if {
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