<!-- markdownlint-disable MD041 -->

A user record may not include a `team` field. Rather than requiring every record
to set one, `object.get` lets the policy supply a fallback. Here the fallback
team is computed from the user's first role, which is the kind of dynamic default
that `object.get` handles well (the `default` keyword only accepts a fixed value).
