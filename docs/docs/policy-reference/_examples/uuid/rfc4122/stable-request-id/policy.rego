package play

# Correlation ID for this request (cache key = request_id).
correlation_id := uuid.rfc4122(input.request_id)

# Reusing the same key yields the same UUID in this evaluation.
decision := {
	"allow": input.action == "read",
	"correlation_id": correlation_id,
	"audit_id": uuid.rfc4122(input.request_id),
}

same_key_match if {
	decision.correlation_id == decision.audit_id
}
