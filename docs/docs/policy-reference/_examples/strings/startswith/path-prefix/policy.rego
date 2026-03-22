package play

# True when a literal path starts with a given prefix.
is_api_path if startswith("/api/v1/users", "/api/v1/")

# Allow the request only if its path is under the /api/v1/ prefix.
allow if startswith(input.path, "/api/v1/")

# Deny by default.
default allow := false
