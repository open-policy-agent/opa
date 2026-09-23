<!-- markdownlint-disable MD041 -->

`uuid.rfc4122` generates a random RFC 4122 UUID. The string argument is a
cache key for the current evaluation: calling the built-in twice with the same
key returns the same UUID, which is useful when a policy attaches one
correlation ID in multiple places in a decision.
