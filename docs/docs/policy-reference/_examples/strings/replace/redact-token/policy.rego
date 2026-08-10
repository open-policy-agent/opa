package play

# Message that would leak a confidential header, then redacted for the caller.
safe_message := replace(
	sprintf("upstream rejected request with %s", [input.headers.authorization]),
	input.headers.authorization,
	"[REDACTED]",
)
