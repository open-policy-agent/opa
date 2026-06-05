<!-- markdownlint-disable MD041 -->

An HTTP method can arrive in any case, so `get`, `GET`, and `Get` should all be
treated the same. This example upper-cases the incoming method before checking
it against an allow-list, so the comparison does not depend on how the caller
happened to write it.
