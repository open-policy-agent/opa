package crypto_digest_verification

# Verify payload integrity using MD5 digest (commonly used for content verification)
payload_json := json.marshal(input.payload)
computed_digest := crypto.md5(payload_json)
digest_valid := computed_digest == input.expected_digest
