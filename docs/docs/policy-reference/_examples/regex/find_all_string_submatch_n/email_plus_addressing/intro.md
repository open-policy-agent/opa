<!-- markdownlint-disable MD041 -->

In the example that follows, we show a policy that uses the
`regex.find_all_string_submatch_n` built-in to extract the 'plus suffix', if
present, from an email address.

This policy ensures that plus addresses are only permitted for use by internal
users to avoid potential abuse.
