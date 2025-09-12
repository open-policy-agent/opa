package play

import rego.v1

default allow := false

allow if {
	# perform checks on the claims...
	verified_claims.sub == "1234567890"
}

verified_claims := claims if {
	[verified, _, claims] := io.jwt.decode_verify(
		input.token,
		{
			"secret": "password",
			"iss": "pki.example.com",
		},
	)

	verified == true
}
