package test

import data.ex

basic = true { ex.loopback = true with input as true; ex.loopback = false with input as false }
negation = true { not ex.loopback with input as false; ex.loopback with input as true }
composite[x] { ex.composite[x] with input.foo as [1, 2, 3, 4] }
vars = x { foo = "hello"; bar = "world"; x = ex.vars with input.foo as foo with input.bar as bar }
conflict = true { ex.loopback with input.foo as "x" with input.foo.bar as "y" }
negation_invalidate[x] { data.a[_] = x; not data.ex.input_eq with input.x as x }
basic_data = true {ex.allow_basic = true with data.a as "testdata"}
map_data_1 = true {ex.allow_merge_1 = true with data.b.v2 as "world"}
map_data_2 = true {ex.allow_merge_2 = true with data.b.v2 as "world" with data.b.v3 as "again"}
data_conflict = true {ex.allow_basic = true with data.a.b as 5}
base_doc_exact_value[x] { data.a.b[x] = 1 with data.a.b as {"c": 1, "d": 2, "e": 1} }
base_doc_any_index[x] { data.a.b[x] with data.a.b as {"c": 1, "d": 2, "e": 1} }
undefined_1 { data.a.b.c with data.a.b as 1 }
undefined_2 { data.l.a with data.l as 1 }
virtual_doc_exact_value[x] { ex.virtual = x with data.a.b as {"c": 1, "d": 2, "e": 1} }
virtual_doc_any_index[x] { ex.virtual[x] with data.a.b as {"c": 1, "d": 2, "e": 1} }
virtual_doc_specific_index = y { y = ex.virtual["c"] with data.a.b as {"c": 1, "d": 2, "e": 1} }
virtual_doc_not_specific_index = true { not ex.virtual["d"] with data.a.b as {"c": 1, "d": 2, "e": 1} }
test_mock_var = y {y = ex.mock_var with ex.mock_var as {"c": 1, "d": 2}}
test_mock_rule {ex.mock_rule with ex.mock_rule as true}
test_rule_chain {ex.allow with data.label.b.c as [1,2,3]}
