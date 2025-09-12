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
            "x": "Uv7zcspR68KLcqZJDV5WYd946uHTeOFhHVi0hAkqkWI",
            "y": "YExfQqSyCo3LlX1K7F9NCTQjcNBNRWAZqsbWtNNHwTU"
        }
    ]
}`

t := time.add_date(
	time.now_ns(),
	100,
	0,
	0,
)

verified_claims := claims if {
	[verified, _, claims] := io.jwt.decode_verify(
		input.token,
		{
			"cert": jwks,
			"iss": "pki.example.com",
			# a time in 2024
			"time": 1720109779675634700,
		},
	)

	verified == true
}

verified_claims_future := claims if {
	[verified, _, claims] := io.jwt.decode_verify(
		input.token,
		{
			"cert": jwks,
			"iss": "pki.example.com",
			# a time in the future
			"time": 4875783401643188000,
		},
	)

	verified == true
}

default allow := false

allow if {
	verified_claims
}

# default is used since token is not verified
default allow_future := false

allow_future if {
	# unset, since the token is not verified
	verified_claims_future
}
