package example

violations contains msg if {
	msg := "hello"
}

allow if {
	count(violations) == 0
}