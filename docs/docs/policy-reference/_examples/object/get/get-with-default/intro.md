<!-- markdownlint-disable MD041 -->

`object.get` looks up a key in an object and returns a supplied default when
the key is missing. Reaching for a key directly with `object[key]` is undefined
when the key is absent, which can silently stop a rule from being defined.
`object.get` avoids that by always returning a value.
