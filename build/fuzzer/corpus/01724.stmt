package ex

loopback = input { true }
composite[x] { input.foo[_] = x; x > 2 }
vars = {"foo": input.foo, "bar": input.bar} { true }
input_eq { input.x = 1 }
allow_basic = true {data.a = "testdata"}
allow_merge_1 = true {data.b = {"v1": "hello", "v2": "world"}}
allow_merge_2 = true {data.b = {"v1": "hello", "v2": "world", "v3": "again"}}
virtual[x] { data.a.b[x] = 1 }
mock_var = {"a": 0, "b": 0}
mock_rule = false {1 = 2}

allow1 {
	data.label.b.c = [1,2,3]
}

allow2 {
	data.label.b.c[x] = 2
}

allow3 {
	data.label.b[x] = 1
}

allow4 {
	data.label.b.c.d[x] = 1
}

allow {
	allow1
	allow2
	not allow3
	not allow4
}
