package jwt

result_hs256 := io.jwt.encode_sign(
    {
        "alg":"HS256",
        "typ":"JWT"
    },
    {},
    {
        "kty":"oct",
        "k":"Zm9v"
    }
)

# Important! - Use the un-encoded plain text secret to verify and decode
result_parts_hs256 := io.jwt.decode_verify(result_hs256, {"secret": "foo"})
result_valid_hs256 := io.jwt.verify_hs256(result_hs256, "foo")
