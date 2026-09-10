<!-- markdownlint-disable MD041 -->

`uuid.rfc4122` generates a random RFC 4122 UUID. The string parameter acts as a
cache key within a single policy evaluation, ensuring multiple references to the
same key yield identical UUIDs during the decision.
