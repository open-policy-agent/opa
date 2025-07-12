package base64_encode

# Encode API credentials for HTTP Basic Auth
username := "admin"
password := "secret123"
credentials := sprintf("%s:%s", [username, password])
auth_header := base64.encode(credentials)

# Encode configuration data for storage
config_data := {"environment": "production", "debug": false}
config_json := json.marshal(config_data)
encoded_config := base64.encode(config_json)

# Encode binary data (simulated as string)
binary_data := "binary file content here"
encoded_binary := base64.encode(binary_data)

# Common Kubernetes use case: encode secret values
secret_value := "my-database-password"
k8s_secret_data := base64.encode(secret_value)