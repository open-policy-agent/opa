<!-- markdownlint-disable MD041 -->

A common policy pattern is rejecting a request whose collection has too
many or too few items, e.g. an Ingress with no rules, or a Pod that asks
for more than `n` containers.

`count` works on arrays, sets, objects, and strings, so the same builtin
covers all four shapes.
