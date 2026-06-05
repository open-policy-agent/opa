<!-- markdownlint-disable MD041 -->

A user record may leave the `team` field unset, and reading it directly would
be undefined and silently stop the rule from being defined. This example fills
the gap with a fallback team computed from the user's own data, the kind of
dynamic default that suits `object.get` better than the `default` keyword, which
only takes a fixed value.
