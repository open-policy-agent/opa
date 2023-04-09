---
title: OAuth2 and OIDC Samples
kind: misc
weight: 1
---

OAuth2 and OpenID Connect are both pervasive technologies in modern identity systems. While verification of JSON web tokens issued by these systems is documented in the [policy reference](https://www.openpolicyagent.org/docs/latest/policy-reference/#token-verification), the policy examples below aim to cover some other common use cases.

## Metadata discovery

Rather than storing endpoints and other metadata as part of policy data, the authorization server metadata endpoint may be queried for this data.

```live:oidc:module
package oidc

issuers := {"https://issuer1.example.com", "https://issuer2.example.com"}

metadata_discovery(issuer) := http.send({
    "url": concat("", [issuers[issuer], "/.well-known/openid-configuration"]),
    "method": "GET",
    "force_cache": true,
    "force_cache_duration_seconds": 86400 # Cache response for 24 hours
}).body

claims := jwt.decode(input.token)[1]
metadata := metadata_discovery(claims.iss)

jwks_endpoint := metadata.jwks_uri
token_endpoint := metadata.token_endpoint
```

## Token verification using JWKS endpoint

Below example uses the keys published at the JWKS endpoint of the authorization server for token verification.

```live:oidc2:module
package oidc

jwks_request(url) := http.send({
    "url": url,
    "method": "GET",
    "force_cache": true,
    "force_cache_duration_seconds": 3600 # Cache response for an hour
})

jwks := jwks_request("https://authorization-server.example.com/jwks").raw_body

verified := io.jwt.verify_rs256(input.token, jwks)
```

### Key rotation

Use the keys published at the JWKS endpoint of the authorization server for token verification, with [key rotation](https://openid.net/specs/openid-connect-core-1_0.html#RotateSigKeys) taken into account.

```live:oidc3:module
package oidc

jwks_request(url) := http.send({
    "url": url,
    "method": "GET",
    "force_cache": true,
    "force_cache_duration_seconds": 3600
})

jwt_unverified := io.jwt.decode(input.token)
jwt_header := jwt_unverified[0]

# Use the key ID (kid) from the token as a cache key - if a new kid is encountered
# we obtain a fresh JWKS object as the keys have likely been rotated.
jwks_url := concat("?", [
    "https://authorization-server.example.com/jwks",
    urlquery.encode_object({"kid": jwt_header.kid}),
])
jwks := jwks_request(jwks_url).raw_body

jwt_verified := jwt_unverified {
    io.jwt.verify_rs256(input.token, jwks)
}

claims_verified := jwt_verified[1]
```

## Token retrieval

Programmatically obtain an OAuth2 access token following the client credentials or resource owner password credential flow.

```live:oauth:module
package oauth2

token := t {
    response := http.send({
        "url": "https://authorization-server.example.com/token",
        "method": "POST",
        "headers": {
            "Content-Type": "application/x-www-form-urlencoded",
            "Authorization": concat(" ", [
                "Basic",
                base64.encode(sprintf("%v:%v", [client_id, client_secret]))
            ]),
        },
        # To use the resource owner password credentials flow, change grant_type
        # to "password" and add username and password parameters to the body
        "raw_body": "grant_type=client_credentials",
        "force_cache": true,
        "force_cache_duration_seconds": 3595, # Given an `expires_in` value of 3600
    })
    response.status_code == 200

    t := response.body.access_token
}
```
