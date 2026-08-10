package play

# Example of a message that would leak the Authorization header.
raw_message := sprintf("upstream rejected request with %s", [input.headers.authorization])

# Same text after redacting the secret for the caller.
safe_message := replace(raw_message, input.headers.authorization, "[REDACTED]")
