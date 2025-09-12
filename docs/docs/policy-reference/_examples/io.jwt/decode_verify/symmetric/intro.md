<!-- markdownlint-disable MD041 -->
:::warning
This example uses a symmetric key to verify the token. This is not recommended for
production use. Please see the examples below using `JWKs` or PEM-encoded certificates more examples.
:::

Sometimes when working with tools like [JWT.io](https://jwt.io) it can be
useful to decode and verify JWT tokens signed with a symmetric key just to
see what the output of `io.jwt.decode_verify()` looks like.

The not-so-secret symmetric key `password` was used to sign the token
provided in the input for the policy below. We can see that the claims in this
example contains `secret` and `iss` only. This means that the validity period
of the token is not checked, nor is the audience or the algorithm used to sign
it.
