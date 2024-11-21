package test["if"]

p if 1 > 0 # shorthand

p if { 1 > 0 } # longhand one line

p if {
  1 > 0 # longhand two lines
}

# same without the comment
p if {
  1 > 0
}

p if { # comment one
  1 > 0 # comment two
}

q[x] = y if {
    y := 10
    x := "ten"
}

r[x] if { x := "set" } # comment
