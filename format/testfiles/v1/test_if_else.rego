package test.if2 # FIXME: refs aren't allowed to contains keywords

p := 1 if { 1 > 0 }
else := 2

q := 1 if { 1 > 0 } else := 2 if { 2 > 1 }

q := 1 if {
  1 > 0
  2 > 1
} else := 2 if { 2 > 1 }

r := 1 if {
  1 > 0
}
else := 2 if {
  2 > 1
}