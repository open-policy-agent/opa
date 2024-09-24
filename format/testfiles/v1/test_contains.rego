package test["contains"]

p contains "foo" if { true }

deny contains msg if {
  msg := "foo"
}
deny contains msg if {msg := "bar" }

# partial objects unchanged
o[k] = v if { k := "ok"; v := "nok" }

foo["contains"] := 42
