package p

first := {"one", "two"}
second := {"two", "three"}

example[msg] {
  msg := (first | second)[_]
}
