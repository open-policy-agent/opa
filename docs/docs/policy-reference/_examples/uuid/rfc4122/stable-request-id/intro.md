<!-- markdownlint-disable MD041 -->

`uuid.rfc4122` generates a random RFC 4122 UUID. The string argument is a
**cache key** within a single policy evaluation: calling the function with the
same key returns the same UUID for the duration of that decision.
