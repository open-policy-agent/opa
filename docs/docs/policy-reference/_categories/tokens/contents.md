:::info
Note that the `io.jwt.verify_XX` built-in methods verify **only** the signature. They **do not** provide any validation for the JWT
payload and any claims specified. The `io.jwt.decode_verify` built-in will verify the payload and **all** standard claims.
:::

The input `string` is a JSON Web Token encoded with JWS Compact Serialization. JWE and JWS JSON Serialization are not supported. If nested signing was used, the `header`, `payload` and `signature` will represent the most deeply nested token.

For `io.jwt.decode_verify`, `constraints` is an object with the following members:

| Name     | Meaning                                                                                                                                                                                              | Required  |
| -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| `cert`   | A PEM encoded certificate, PEM encoded public key, or a JWK key (set) containing an RSA or ECDSA public key.                                                                                         | See below |
| `secret` | The secret key for HS256, HS384 and HS512 verification.                                                                                                                                              | See below |
| `alg`    | The JWA algorithm name to use. If it is absent then any algorithm that is compatible with the key is accepted.                                                                                       | Optional  |
| `iss`    | The issuer string. If it is present the only tokens with this issuer are accepted. If it is absent then any issuer is accepted.                                                                      | Optional  |
| `time`   | The time in nanoseconds to verify the token at. If this is present then the `exp` and `nbf` claims are compared against this value. If it is absent then they are compared against the current time. | Optional  |
| `aud`    | The audience that the verifier identifies with. If this is present then the `aud` claim is checked against it. **If it is absent then the `aud` claim must be absent too.**                          | Optional  |

Exactly one of `cert` and `secret` must be present. If there are any
unrecognized constraints then the token is considered invalid.

#### Token Verification Examples

The examples below use the following token:

```rego
package jwt

es256_token := "eyJ0eXAiOiAiSldUIiwgImFsZyI6ICJFUzI1NiJ9.eyJuYmYiOiAxNDQ0NDc4NDAwLCAiaXNzIjogInh4eCJ9.lArczfN-pIL8oUU-7PU83u-zfXougXBZj6drFeKFsPEoVhy9WAyiZlRshYqjTSXdaw8yw2L-ovt4zTUZb2PWMg"
```

<RunSnippet id="token.rego"/>

#### Using JWKS

This example shows a two-step process to verify the token signature and then decode it for
further checks of the payload content. This approach gives more flexibility in verifying only
the claims that the policy needs to enforce.

```rego
package jwt

jwks := `{
    "keys": [{
        "kty":"EC",
        "crv":"P-256",
        "x":"z8J91ghFy5o6f2xZ4g8LsLH7u2wEpT2ntj8loahnlsE",
        "y":"7bdeXLH61KrGWRdh7ilnbcGQACxykaPKfmBccTHIOUo"
    }]
}`
```

<RunSnippet id="jwks.rego"/>
<PlaygroundExample files="#jwks.rego #token.rego" dir={require.context("../../_examples/tokens/verify/jwks")} />

<PlaygroundExample files="#jwks.rego #token.rego" dir={require.context("../../_examples/tokens/verify/jwks_single")} />

#### Using PEM encoded X.509 Certificate

The following examples will demonstrate verifying tokens using an X.509 Certificate
defined as:

```rego
package jwt

cert := `-----BEGIN CERTIFICATE-----
MIIBcDCCARagAwIBAgIJAMZmuGSIfvgzMAoGCCqGSM49BAMCMBMxETAPBgNVBAMM
CHdoYXRldmVyMB4XDTE4MDgxMDE0Mjg1NFoXDTE4MDkwOTE0Mjg1NFowEzERMA8G
A1UEAwwId2hhdGV2ZXIwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAATPwn3WCEXL
mjp/bFniDwuwsfu7bASlPae2PyWhqGeWwe23Xlyx+tSqxlkXYe4pZ23BkAAscpGj
yn5gXHExyDlKo1MwUTAdBgNVHQ4EFgQUElRjSoVgKjUqY5AXz2o74cLzzS8wHwYD
VR0jBBgwFoAUElRjSoVgKjUqY5AXz2o74cLzzS8wDwYDVR0TAQH/BAUwAwEB/zAK
BggqhkjOPQQDAgNIADBFAiEA4yQ/88ZrUX68c6kOe9G11u8NUaUzd8pLOtkKhniN
OHoCIHmNX37JOqTcTzGn2u9+c8NlnvZ0uDvsd1BmKPaUmjmm
-----END CERTIFICATE-----`
```

<RunSnippet id="cert.rego"/>

<PlaygroundExample files="#cert.rego #token.rego" dir={require.context("../../_examples/tokens/verify/cert")} />

<PlaygroundExample files="#cert.rego #token.rego" dir={require.context("../../_examples/tokens/verify/cert_single")} />

#### Round Trip - Sign and Verify

These examples show how to encode a token, verify, and decode it with the different options available.

<PlaygroundExample dir={require.context("../../_examples/tokens/verify/sign_raw")} />

<PlaygroundExample dir={require.context("../../_examples/tokens/verify/sign")} />

:::info
Note that the resulting encoded token is different from the first example using
`io.jwt.encode_sign_raw`. The reason is that the `io.jwt.encode_sign` function
is using canonicalized formatting for the header and payload whereas
`io.jwt.encode_sign_raw` does not change the whitespace of the strings passed
in. The decoded and parsed JSON values are still the same.
:::

<!--
    DO NOT MOVE THESE!
    Would like them at the top but MDX parsing be weird and doing that breaks everything
-->

import PlaygroundExample from '@site/src/components/PlaygroundExample';
import RunSnippet from '@site/src/components/RunSnippet';
