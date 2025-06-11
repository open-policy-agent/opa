package jwt

result := io.jwt.encode_sign_raw(
    `{"typ":"JWT","alg":"HS256"}`,
     `{"iss":"joe","exp":1300819380,"http://example.com/is_root":true}`,
    `{"kty":"oct","k":"AyM1SysPpbyDfgZld3umj1qzKObwVMkoqQ-EstJQLr_T-1qS0gZH75aKtMN3Yj0iPS4hcgUuTwjAzZr1Z9CAow"}`
)
