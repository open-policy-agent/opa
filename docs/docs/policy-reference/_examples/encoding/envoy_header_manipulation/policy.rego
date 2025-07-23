package envoy.authz

# Extract credentials from request headers (common pattern for upstream auth)
username := input.attributes.request.http.headers["x-username"]
password := input.attributes.request.http.headers["x-password"]

# Default deny - only allow if credentials are provided
default allow := false

allow if {
    username != ""
    password != ""
}

# Add Authorization header to downstream request using base64 encoding
response_headers_to_add := {
    "Authorization": sprintf("Basic %s", [base64.encode(sprintf("%s:%s", [username, password]))])
}
