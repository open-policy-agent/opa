<!-- markdownlint-disable MD041 -->
Most commonly, `if` is used to create boolean rules where if any rule head is
true, then the whole rule is true. In this simple example, when both:

- the `input.role` field is present,
- and set to the value of "admin"

Then, the `allow` rule will be `true`. If either of these conditions is not met,
the rule will be `false`.
