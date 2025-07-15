package crypto_sha256

# Verify file integrity by comparing SHA256 hashes
file_content := "Hello, OPA!"
expected_hash := "23883ac329c294e797c539e46f35abfd1e17a7add89e9eea09943bf9e94d6435"
actual_hash := crypto.sha256(file_content)
hash_matches := actual_hash == expected_hash