package base64_encode

# Encode credentials for HTTP Basic Authentication
username := "admin"
password := "secret123"
credentials := sprintf("%s:%s", [username, password])
auth_header := base64.encode(credentials)