package p

single_line_with if {
  fn(1)   with input.a as "a"
}

multi_line_with if {
    fn(1) with input.a as "a"
                with input.b as "b"
            with input.c as {
                "foo": "bar",
            }
                with input.d as [
                    1,
                    2,
                    3]
}

multi_line_with_all_indented if {
    fn(1)
        with input.a as "a"
        with input.b as "b"
        with input.c as "c"
}

multi_line_with_all_indented_messy if {
    fn(1)
with input.a as "a"
        with input.b as "b"
    with input.c as "c"
}

mixed_new_lines_with if {
    true with input.a as "a"
      with input.b as "b" with input.c as "c"
      with input.d as "d"
}

mock_f(_) = 123

func_replacements if {
    count(array.concat(input.x, [])) with input.x as "foo"
    with array.concat as true
    with count as mock_f
}

original(x) = x+1

more_func_replacements if {
    original(1) with original as mock_f
    original(1) with original as 1234
}

lone_with_multiline_call if {
	foo(
		arg_a,
		bar(arg_b, arg_c),
		"some message",
	) with input.x as false
}

lone_with_every_block if {
	every item in items {
		check(item)
	} with data.x as mock
}