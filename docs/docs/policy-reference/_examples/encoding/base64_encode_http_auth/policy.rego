package base64_encode_http_auth

# Envoy external auth response with Basic Auth header
allow := true

response_headers_to_add := [
    {
        "header": {
            "key": "Authorization",
            "value": sprintf("Basic %s", [base64.encode(sprintf("%s:%s", [input.username, input.password]))])
        }
    }
]
