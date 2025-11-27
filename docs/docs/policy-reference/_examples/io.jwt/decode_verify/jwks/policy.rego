package play

import rego.v1

jwks := `{
  "keys": [
    {
      "alg": "ES256",
      "crv": "P-256",
      "kid": "my-key-id",
      "kty": "EC",
      "use": "sig",
      "x": "iTV4PECbWuDaNBMTLmwH0jwBTD3xUXR0S-VWsCYv8Gc",
      "y": "-Cnw8d0XyQztrPZpynrFn8t10lyEb6oWqWcLJWPUB5A"
    }
  ]
}`

allow if {
	# perform checks on the verified claims...
	verified_claims
}

verified_claims := claims if {
	[verified, _, claims] := io.jwt.decode_verify(
		input.token,
		{
			"cert": jwks,
			"iss": "pki.example.com",
		},
	)

	verified == true
}
