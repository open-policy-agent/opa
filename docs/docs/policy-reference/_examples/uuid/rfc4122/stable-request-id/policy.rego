package play

# One correlation ID for this request (cache key = request_id).
correlation_id := uuid.rfc4122(input.request_id)

# Decision uses the same key twice so allow and audit share one ID.
decision := {
	"allow": input.action == "read",
	"correlation_id": correlation_id,
	"audit_id": uuid.rfc4122(input.request_id),
}

# same key => same UUID within this evaluation
same_key_match if {
	decision.correlation_id == decision.audit_id
}
