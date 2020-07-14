// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package bundle provide helpers that assist in the creating a signed bundle
package bundle

import (
	"encoding/json"

	"github.com/open-policy-agent/opa/internal/jwx/jwa"
	"github.com/open-policy-agent/opa/internal/jwx/jws"
)

// GenerateSignedToken generates a signed token given the list of files to be
// included in the payload and the bundle signing config. The keyID if non-empty,
// represents the value for the "keyid" claim in the token
func GenerateSignedToken(files []FileInfo, sc *SigningConfig, keyID string) (string, error) {
	payload, err := generatePayload(files, sc, keyID)
	if err != nil {
		return "", err
	}

	privKey, err := sc.GetPrivateKey()
	if err != nil {
		return "", err
	}

	// generate signed token
	token, err := jws.SignWithOption(payload, jwa.SignatureAlgorithm(sc.Algorithm), privKey)
	if err != nil {
		return "", err
	}
	return string(token), nil
}

func generatePayload(files []FileInfo, sc *SigningConfig, keyID string) ([]byte, error) {
	payload := make(map[string]interface{})
	payload["files"] = files

	if sc.ClaimsPath != "" {
		claims, err := sc.GetClaims()
		if err != nil {
			return nil, err
		}

		for claim, value := range claims {
			payload[claim] = value
		}
	} else {
		if keyID != "" {
			payload["keyid"] = keyID
		}
	}
	return json.Marshal(payload)
}
