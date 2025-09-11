export const Ex1Title = `Check If a User Is an Admin`;

export const Ex1Intro = `
Admin status is often encoded in a [JWT](http://jwt.io) (JSON Web Token).
This example shows how to extract information from a JWT and make an
authorization decision based on it. The example uses an insecure secret
to verify the JWT for brevity.
`;

export const Ex1Outro = `
**Note:** Verifying JWTs with a hard-coded secret is insecure and is used here for example only,
please refer to the OPA documentation
on [Token Verification](/docs/policy-reference/builtins/tokens)
for more information on how to securely verify JWTs.
`;

export const Ex1Rego = `package example

claims := io.jwt.decode(input.token)[1] if {
    io.jwt.verify_hs256(input.token, "pa$$w0rd")
}

default allow := false

allow if "admin" in claims.roles
`;

export const Ex1Python = `import jwt # pip install pyjwt

def allow(token):
    try:
        payload = jwt.decode(
            token, "pa$$w0rd", algorithms=["HS256"])
    except jwt.PyJWTError as e:
        return False

    if not "roles" in payload:
        return False

    return "admin" in payload["roles"]
`;

export const Ex1Java = `package com.example.app;

import io.jsonwebtoken.Claims;
import io.jsonwebtoken.ExpiredJwtException;
import io.jsonwebtoken.Jwt;
import io.jsonwebtoken.Jwts;
import io.jsonwebtoken.MalformedJwtException;
import io.jsonwebtoken.SignatureException;
import io.jsonwebtoken.UnsupportedJwtException;
import java.util.List;
import java.util.Map;

public class Authorization {
    public static boolean allow(String token) {
        Claims claims = null;
        try {
            claims = Jwts.parser()
                         .setSigningKey("cGEkJHcwcmQ=") // b64 "pa$$w0rd"
                         .build()
                         .parseSignedClaims(token)
                         .getPayload();
        } catch (SignatureException
            | ExpiredJwtException
            | UnsupportedJwtException
            | MalformedJwtException e) {
            return false;
        } catch (Exception e) {
            return false;
        }

        Object rolesObj = claims.get("roles");
        if (rolesObj instanceof List) {
            List<String> roles = (List<String>) rolesObj;
            return roles.contains("admin");
        }

        return false;
    }
}
`;

export const Ex1Go = `package main

import (
    "fmt"

    "github.com/golang-jwt/jwt"
)

func allow(tokenString string) (bool, error) {
    token, err := jwt.Parse(
        tokenString,
        func(token *jwt.Token) (any, error) {
            if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
            }

            return []byte("pa$$w0rd"), nil
        },
    )
    if err != nil {
        return false, fmt.Errorf("failed to parse token: %v", err)
    }

    if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
        roles, ok := claims["roles"].([]any)
        if !ok {
            return false, nil
        }
        if slices.Contains(roles, "admin") {
            return true, nil
        }
    }

    return false, nil
}
`;
