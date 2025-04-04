package a

p[
{"a": #
"b"} #
] := true

value := {"a":
{"b":
{"c":
"d"}}}

value := {"a":  # test 1
"b"} # test 2

value := {"a":  # test 1
"b"}

value := {"a":
{"b":
{"c":
"d"}}}

value := {"a": # this is
{"b": # my ridiculous
{"c": # way of
"d"}}} # commenting code

p := {"a":  #
"b"} if { #
	str := "my \n string"
}

value := {"a":
{"b":
{"c":
"d"}}}

f(_) := value if {
	value := {"a": # this is
        {"b": # my ridiculous
        {"c": # way of
        "d"}}} # commenting code
}

p := value if {
	value := {"a": # this is
        {"b": # my ridiculous
        {"c": # way of
        "d"}}} # commenting code
}

p := x if {
	x := [v | v := {"a": # this is
        {"b": # my ridiculous
        {"c": # way of
        "d"}}} # commenting code
    ]
}

p if {
	every x in input.foo {
    	x == {"a": # this is
            {"b": # my ridiculous
            {"c": # way of
            "d"}}} # commenting code
    }
}

value contains {"a":  #
"b"} #

p := {"a":  #
str} if {
	str := "my \n string"
}

p := {"a":
str} if {
    #
	str := "my \n string"
}

authorize := "allow" if {
	value := {"a": # this is
    {"b": # my ridiculous
    {"c": # way of
    "d"}}} # commenting code
    input.user == "superuser"           # allow 'superuser' to perform any operation.
} else := "deny" if {
    value := {"a": # this is
    {"b": # my ridiculous
    {"c": # way of
    "d"}}} # commenting code
    input.path[0] == value              # disallow 'admin' operations...
    input.source_network == "external"  # from external networks.
} # ... more rules

p[
{"a": #
"b"} #
] := true

p.foo.bar[
{"a": #
"b"} #
] := true

p[
{"a": #
"b"} #
][
{"c": #
"d"} #
] := true

p if {
    x := {"a": # do
          "b"} # re
    1 + 2      ==       3
    y := {"c": # mi
          "d"} # fa
    x != y
}

p[x].r := y if {
	x := "q"
	y := 1
	    y := {"c": # hello
"d"} # world



    y := 1
}