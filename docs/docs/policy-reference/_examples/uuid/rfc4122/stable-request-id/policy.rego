package play

# Generate a correlation ID for this policy decision using the request ID as a cache key.
correlation_id := uuid.rfc4122(input.request_id)

# Return authorization decision along with the audit correlation ID.
decision := {
	"allow": input.action == "read",
	"correlation_id": correlation_id,
}
