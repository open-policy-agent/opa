package play

import rego.v1

cert := `-----BEGIN CERTIFICATE-----
MIIBMzCB2aADAgECAgEBMAoGCCqGSM49BAMCMBIxEDAOBgNVBAoTB0V4YW1wbGUw
HhcNMDkxMTEwMjMwMDAwWhcNMTAxMTEwMjMwMDAwWjASMRAwDgYDVQQKEwdFeGFt
cGxlMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAERgI9efDtzy0QubqeMgYPyD+k
GWtw2JueCpORUB0hOecwnO5HjPiZ3OsE5wIvwzt8fJzbZFHBoKx5GfbUCiOlpKMg
MB4wDgYDVR0PAQH/BAQDAgWgMAwGA1UdEwEB/wQCMAAwCgYIKoZIzj0EAwIDSQAw
RgIhAIQNOE/SCRr2pkW4XCGIaHuNO6oXHDp/HThPxaHfyTmPAiEA8uPj91awzzdW
xOI1W2BMAnR1VlCHTaaoGCaWUjTo6Sc=
-----END CERTIFICATE-----
`

default allow := false

allow if {
	# perform additional checks as required
	verified_claims
}

verified_claims := claims if {
	[verified, _, claims] := io.jwt.decode_verify(
		input.token,
		{
			"cert": cert,
			"iss": "pki.example.com",
		},
	)

	verified == true
}
