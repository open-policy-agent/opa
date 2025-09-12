package play

import rego.v1

example_object := {
	"read": true,
	"write": true,
	"delete": false,
	"create": false,
}

permission_list contains permission if {
	some permission, value in example_object

	value == true
}
