<!-- markdownlint-disable MD041 -->

`endswith` checks whether a string ends with a given suffix. Use it for file
extensions, email domains, or other trailing markers where `contains` would
match in the wrong place.

This example only allows uploads whose names end with `.json` or `.yaml`.
