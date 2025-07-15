package base64_encode_http_auth

# Create Basic Auth header for downstream request (Envoy use case)
basic_auth_header := sprintf("Basic %s", [base64.encode(sprintf("%s:%s", [input.username, input.password]))])

# Individual encoded values for reference
encoded_username := base64.encode(input.username)
encoded_password := base64.encode(input.password)
