package crypto_sha256

# Verify file integrity using SHA256 hash
file_content := "Hello, OPA!"
expected_hash := "23883ac329c294e797c539e46f35abfd1e17a7add89e9eea09943bf9e94d6435"
actual_hash := crypto.sha256(file_content)

# Verify container image digest
container_image := "nginx:1.21"
image_content := "container layer data"
image_digest := crypto.sha256(image_content)

# Check if computed hash matches expected
hash_matches := actual_hash == expected_hash

# Common DevOps use case: verify configuration file hasn't changed
config_file := `{"database": {"host": "localhost", "port": 5432}}`
config_hash := crypto.sha256(config_file)