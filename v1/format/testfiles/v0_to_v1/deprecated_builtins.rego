package test

p {
	any([true, false])
	all([true, false])
	cast_array(["foo", "bar"])
	cast_boolean(true)
	cast_null(null)
	cast_object({"foo": "bar"})
	cast_set({"foo", "bar"})
	cast_string("foo")
	net.cidr_overlap("127.0.0.1/24", "127.0.0.64/26")
	re_match("f.o", "foo")
	set_diff({"a", "b", "c"},{"b", "c"})
}

q := any([true, false])

r[any([true, false])]

s[any([true, false])][all([true, false])]
