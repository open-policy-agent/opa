package play

# "/teams/payments/deploy" -> ["", "teams", "payments", "deploy"]
parts := split(input.path, "/")
team := parts[2]

default allow := false

allow if team == "payments"
