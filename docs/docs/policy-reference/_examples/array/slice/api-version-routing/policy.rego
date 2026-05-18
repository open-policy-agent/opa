package play

# Split a request path into segments.
# e.g. "/api/v1/users/123" → ["api", "v1", "users", "123"]
path_segments := split(trim_left(input.path, "/"), "/")

# The API version is always the second segment (index 1).
api_version := path_segments[1]

# The first two segments identify the route, regardless of how long the path is.
route_prefix := array.slice(path_segments, 0, 2)

# Deny requests that do not target a supported API version.
deny contains msg if {
	api_version != "v1"
	msg := sprintf("unsupported API version %q; only v1 is allowed", [api_version])
}
