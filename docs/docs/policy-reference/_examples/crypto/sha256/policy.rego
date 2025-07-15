package crypto_sha256

# Verify data integrity by comparing provided hash with computed hash
actual_hash := crypto.sha256(input.file_content)
hash_verified := actual_hash == input.expected_hash
