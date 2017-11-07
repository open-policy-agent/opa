// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package authz

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
)

type dataSetProfile struct {
	numTokens int
	numPaths  int
}

func TestAuthz(t *testing.T) {

	profile := dataSetProfile{
		numTokens: 1000,
		numPaths:  10,
	}

	ctx := context.Background()
	data := generateDataset(profile)
	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	compiler := ast.NewCompiler()
	module := ast.MustParseModule(policy)

	compiler.Compile(map[string]*ast.Module{"": module})
	if compiler.Failed() {
		t.Fatalf("Unexpected error(s): %v", compiler.Errors)
	}

	input, expected := generateInput(profile, forbidPath)

	r := rego.New(
		rego.Compiler(compiler),
		rego.Store(store),
		rego.Transaction(txn),
		rego.Input(input),
		rego.Query("data.restauthz.allow"),
	)

	rs, err := r.Eval(ctx)

	if err != nil {
		t.Fatalf("Unexpected error(s): %v", err)
	}

	if len(rs) != 1 || util.Compare(rs[0].Expressions[0].Value, expected) != 0 {
		t.Fatalf("Unexpected result: %v", rs)
	}
}

type inputMode int

const (
	forbidIdentity = iota
	forbidPath     = iota
	forbidMethod   = iota
	allow          = iota
)

func generateInput(profile dataSetProfile, mode inputMode) (interface{}, interface{}) {

	var input string
	var allow bool

	switch mode {
	case forbidIdentity:
		input = fmt.Sprintf(`
		{
			"token_id": "deadbeef",
			"path": %q,
			"method": "GET"
		}
		`, generateRequestPath(profile.numPaths-1))
	case forbidPath:
		input = fmt.Sprintf(`
		{
			"token_id": %q,
			"path": %q,
			"method": "GET"
		}`, generateTokenID(profile.numTokens-1), "/api/v1/resourcetype-deadbeef/deadbeefresourceid")
	case forbidMethod:
		input = fmt.Sprintf(`
		{
			"token_id": %q,
			"path": %q,
			"method": "DEADBEEF"
		}
		`, generateTokenID(profile.numTokens-1), generateRequestPath(profile.numPaths-1))
	default:
		input = fmt.Sprintf(`
		{
			"token_id": %q,
			"path": %q,
			"method": "GET"
		}
		`, generateTokenID(profile.numTokens-1), generateRequestPath(profile.numPaths-1))
		allow = true
	}

	return util.MustUnmarshalJSON([]byte(input)), allow
}

func generateDataset(profile dataSetProfile) map[string]interface{} {
	return map[string]interface{}{
		"restauthz": map[string]interface{}{
			"tokens": generateTokensJSON(profile),
		},
	}
}

func generateTokensJSON(profile dataSetProfile) interface{} {
	tokens := generateTokens(profile)
	bs, err := json.Marshal(tokens)
	if err != nil {
		panic(err)
	}
	return util.MustUnmarshalJSON(bs)
}

type Token struct {
	ID            string         `json:"id"`
	AuthzProfiles []AuthzProfile `json:"authz_profiles"`
}

type AuthzProfile struct {
	Path    string   `json:"path"`
	Methods []string `json:"methods"`
}

func generateTokens(profile dataSetProfile) map[string]Token {
	tokens := map[string]Token{}
	for i := 0; i < profile.numTokens; i++ {
		token := generateToken(profile, i)
		tokens[token.ID] = token
	}
	return tokens
}

func generateToken(profile dataSetProfile, i int) Token {
	token := Token{
		ID:            generateTokenID(i),
		AuthzProfiles: generateAuthzProfiles(profile),
	}
	return token
}

func generateAuthzProfiles(profile dataSetProfile) []AuthzProfile {
	profiles := make([]AuthzProfile, profile.numPaths)
	for i := 0; i < profile.numPaths; i++ {
		profiles[i] = generateAuthzProfile(profile, i)
	}
	return profiles
}

func generateAuthzProfile(profile dataSetProfile, i int) AuthzProfile {
	return AuthzProfile{
		Path: generateAuthzPath(i),
		Methods: []string{
			"POST",
			"GET",
		},
	}
}

func generateTokenID(suffix int) string {
	return fmt.Sprintf("token-%d", suffix)
}

func generateAuthzPath(i int) string {
	return fmt.Sprintf("/api/v1/resourcetype-%d/*", i)
}

func generateRequestPath(i int) string {
	return fmt.Sprintf("/api/v1/resourcetype-%d/somefakeresourceid000000111111", i)
}

const policy = `package restauthz

import data.restauthz.tokens

default allow = false

allow {
	tokens[input.token_id] = token
	token.authz_profiles[_] = authz
	re_match(authz.path, input.path)
	authz.methods[_] = input.method
}`
