package p

single_line_with {
  fn(1)   with input.a as "a"
}

multi_line_with {
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

mixed_new_lines_with {
    true with input.a as "a"
      with input.b as "b" with input.c as "c"
      with input.d as "d"
}

mock_f(_) = 123

func_replacements {
    count(array.concat(input.x, [])) with input.x as "foo"
    with array.concat as true
    with count as mock_f
}

original(x) = x+1

more_func_replacements {
    original(1) with original as mock_f
    original(1) with original as 1234
}