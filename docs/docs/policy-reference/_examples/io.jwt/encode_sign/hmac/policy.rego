package jwt

result := io.jwt.encode_sign({
    "typ": "JWT",
    "alg": "HS256"
}, {
    "iss": "joe",
    "exp": 1300819380,
    "aud": ["bob", "saul"],
    "http://example.com/is_root": true,
    "privateParams": {
        "private_one": "one",
        "private_two": "two"
    }
}, {
    "kty": "oct",
    "k": "AyM1SysPpbyDfgZld3umj1qzKObwVMkoqQ-EstJQLr_T-1qS0gZH75aKtMN3Yj0iPS4hcgUuTwjAzZr1Z9CAow"
})
