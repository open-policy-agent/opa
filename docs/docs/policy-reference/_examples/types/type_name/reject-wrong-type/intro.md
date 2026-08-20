<!-- markdownlint-disable MD041 -->

`type_name` returns the Rego type of a value as a string (`"string"`,
`"number"`, `"object"`, and so on). It is useful when input may arrive with
the wrong JSON type and you want a clear deny message.
