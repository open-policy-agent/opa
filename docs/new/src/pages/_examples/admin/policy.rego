package play

# by default, no one is allowed
default allow := false

# however, those with the role "admin"
# are allowed
allow if input.role == "admin"
