package p

first := {"one", "two"}
second := {"two", "three"}

example contains msg if {
  msg := (first | second)[_]
}
