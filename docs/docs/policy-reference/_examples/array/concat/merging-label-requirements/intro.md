<!-- markdownlint-disable MD041 -->

`array.concat` joins two arrays in order. A common use is building a complete
validation list by merging a shared base set of rules with environment-specific
additions, then iterating the result in a single `deny` rule.
