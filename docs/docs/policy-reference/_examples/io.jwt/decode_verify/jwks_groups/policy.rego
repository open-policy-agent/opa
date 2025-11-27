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
      "x": "hV6Tel-bKwtnfgLAn9aPCe24WgKpnoZwDXKbAKdjUV4",
      "y": "xmIK_KmwlJ_jrWAaTvdfvqvAYuxDH_2e4dbIzPQmnFM"
    }
  ]
}`

default allow := false

allow if {
	"developers" in verified_claims.groups
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
