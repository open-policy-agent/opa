<!-- markdownlint-disable MD041 -->

`startswith` returns `true` when the first string
begins with the second string.

A common use case in policies is checking that an HTTP request path falls under
a given prefix — for example, to allow access only to paths under `/api/v1/`.
