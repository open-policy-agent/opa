package test

negated_scalar_element if not {1}

negated_ref_element if not {input.x}

negated_multiple_elements if not {1, 2}

negated_set_element if not {{1}}

negated_with_modifier if not {1} with input as 1

negated_parenthesized if not ({1, 2})

parenthesized if ({1, 2})

parenthesized_ref_element if ({input.x})

empty_set if set()

negated_empty_set if not set()

fn(x) if not {x}

collection contains 1 if not {1}

a.b.c if not {1}

with_else := 1 if not {1} else := 2 if true
