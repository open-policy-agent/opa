<!-- markdownlint-disable MD041 -->

`is_string` (and the related `is_*` helpers) test a value's type and return a
boolean. Prefer them in rule bodies when you only need a yes/no check rather
than the type name itself.
