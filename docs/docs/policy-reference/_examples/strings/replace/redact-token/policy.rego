package play

# Hide the token if this message is shown to the user.
safe_message := replace(input.message, input.token, "[REDACTED]")
