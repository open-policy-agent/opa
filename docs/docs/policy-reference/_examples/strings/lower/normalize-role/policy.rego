package play

role := lower(input.role)

default allow := false

allow if role in data.admin_roles
