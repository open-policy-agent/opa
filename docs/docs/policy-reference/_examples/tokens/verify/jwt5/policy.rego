package jwt

raw_result_hs256 := io.jwt.encode_sign_raw(
    `{"alg":"HS256","typ":"JWT"}`,
    `{}`,
    `{"kty":"oct","k":"Zm9v"}`  	# "Zm9v" == base64url.encode_no_pad("foo")
)

# Important! - Use the un-encoded plain text secret to verify and decode
raw_result_valid_hs256 := io.jwt.verify_hs256(raw_result_hs256, "foo")
raw_result_parts_hs256 := io.jwt.decode_verify(raw_result_hs256, {"secret": "foo"})
