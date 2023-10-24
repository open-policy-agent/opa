package example

import rego.v1

# R1: constant
a := 1

# R2: set
b contains "c"

# R3: boolean
c.d.e := true

# R4: set
d contains x if {
	x := "e"
}

# R5: boolean
e.f[x] if {
	x := "g"
}
