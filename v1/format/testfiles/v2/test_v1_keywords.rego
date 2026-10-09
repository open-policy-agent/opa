package test

# The v1 keywords need no import in v2.
p contains x if {
	some x in input.xs
	every y in input.ys { y != x }
}

q if { 1 in input.xs }

default r := false
