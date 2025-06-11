package jwt

result.verify := io.jwt.verify_es256(es256_token, jwks) # Verify the token with the JWKS
result.payload := io.jwt.decode(es256_token)            # Decode the token
result.check := result.payload[1].iss == "xxx"          # Ensure the issuer (`iss`) claim is the expected value
