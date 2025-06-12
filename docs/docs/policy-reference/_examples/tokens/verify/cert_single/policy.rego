package jwt

result := [valid, header, payload] if {
  [valid, header, payload] := io.jwt.decode_verify(es256_token, {
      "cert": cert,
      "iss": "xxx",
  })
}
