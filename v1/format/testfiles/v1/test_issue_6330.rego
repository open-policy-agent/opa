package a

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