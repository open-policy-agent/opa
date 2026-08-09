package play

# Turn "user_alice" into "alice" for a stable subject key.
subject := replace(input.user_id, "user_", "")

default allow := false

allow if subject in data.allowed_users
