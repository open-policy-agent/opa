<!-- markdownlint-disable MD041 -->

`uuid.rfc4122` generates a random RFC 4122 UUID. The string argument is a
cache key for the current evaluation: calling the built-in twice with the same
key returns the same UUID. That is useful when a decision needs one
correlation ID on both the response path and an audit field.
