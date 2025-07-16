package crypto_sha256

# Verify signed payload by checking JSON representation matches supplied signature
payload_json := json.marshal(input.payload)
computed_hash := crypto.sha256(payload_json)
signature_valid := computed_hash == input.signature
