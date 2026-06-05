<!-- markdownlint-disable MD041 -->

A workload starts from a fixed set of defaults, and each request supplies only
the fields it wants to change. This example layers the request's overrides on
top of those defaults so the merged result keeps every default that was not
overridden, with the request winning on any key it sets.
