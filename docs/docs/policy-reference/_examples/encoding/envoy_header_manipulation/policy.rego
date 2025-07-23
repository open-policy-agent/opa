package envoy.authz

# Extract credentials from request header
headers := input.attributes.request.http.headers

username := headers["x-username"]
password := headers["x-password"]

# Default deny - only allow if credentials are provided
default allow := false

allow if {
    username != ""
    password != ""
}

# Add Authorization header to downstream request using base64 encoding
response_headers_to_add := {
    "Authorization": sprintf(
        "Basic %s", [
            base64.encode(sprintf(
                "%s:%s", [username, password])
            )
        ]
    )
}
