package test.contains2 # FIXME: refs aren't allowed to contains keywords

p contains "foo" if { true }

deny contains msg if {
  msg := "foo"
}
deny contains msg if {msg := "bar" }

# partial objects unchanged
o[k] = v if { k := "ok"; v := "nok" }
