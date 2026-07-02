<!-- markdownlint-disable MD041 -->

So far, all the examples on this page have used the `constraints` parameter
to specify the claims that should be checked and their values. The functionality
covered by `constraints` represents the core checks when verifying a JWT token.

JWT tokens however can contain a lot more information than just the claims
specified in the `constraints` parameter. This example makes an additional check
in the `allow` rule to ensure that the token's groups claim contains the
expected value.
