package test.contains
import future.keywords.contains

p contains "foo" { true }

deny contains msg {
  msg := "foo"
}
deny[msg] {msg := "bar" }

# partial objects unchanged
o[k] = v { k := "ok"; v := "nok" }
