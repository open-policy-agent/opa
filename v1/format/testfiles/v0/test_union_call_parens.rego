package test

# The union built-in's infix form (`|`) is comprehension syntax in collection
# literals and comprehension terms, so it must be parenthesized there.

set_element := {or(x, y)}

array_element := [or(x, y)]

object_value := {"k": or(x, y)}

object_key := {or(x, y): "v"}

nested_set_element := {{or(x, y)}}

nested_array_element := [[or(x, y)]]

array_element_in_object_value := {"k": [or(x, y)]}

union_of_unions := {or(or(x, y), z)}

lone_set_term_body { {or(x, y)} }

partial_set_element[{or(x, y)}] { true }

multiple_set_elements := {or(x, y), z}

multiple_array_elements := [or(x, y), z]

multiple_object_entries := {"k": or(x, y), "j": z}

multiline_set_element := {
	or(x, y),
}

array_comprehension_term := [or(a, b) | c]

set_comprehension_term := {or(a, b) | c}

object_comprehension_value := {k: or(a, b) | c}

object_comprehension_key := {or(a, b): v | c}

object_comprehension_key_and_value := {or(a, b): or(c, d) | e}

# Parens in the source are preserved in every position where they're required.

parens_set_element := {(x | y)}

parens_array_element := [(x | y)]

parens_object_value := {"k": (x | y)}

parens_object_key := {(x | y): "v"}

parens_nested_set_element := {{(x | y)}}

parens_nested_array_element := [[(x | y)]]

parens_union_of_unions := {((x | y) | z)}

parens_redundant := {((x | y))}

parens_lone_set_term_body { {(x | y)} }

parens_partial_set_element[{(x | y)}] { true }

parens_multiple_set_elements := {(x | y), z}

parens_multiline_set_element := {(
	x | y
)}

parens_array_comprehension_term := [(a | b) | c]

parens_set_comprehension_term := {(a | b) | c}

parens_object_comprehension_value := {k: (a | b) | c}

parens_object_comprehension_key := {(a | b): v | c}

parens_object_comprehension_key_and_value := {(a | b): (c | d) | e}

# Unaffected: `|` is unambiguous outside of collection literals and
# comprehension terms, and `&` is never comprehension syntax.

union_outside_collection := or(x, y)

intersection_in_set := {and(x, y)}

union_as_call_argument := f(or(a, b))

union_as_ref_operand := q[or(a, b)]

union_in_comprehension_body := [x | y := or(a, b)]
