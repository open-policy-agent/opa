package assignments

# default value assignment
default a := 1

# rule
b := 2

# else keyword
c := 3 if {
	false
} else := 4 if {
	true
}

# partial rule
d[msg] := 5 if {
	msg = [1, 2, 3][_]
}

# function return value
e := f(6)

f(x) := x
