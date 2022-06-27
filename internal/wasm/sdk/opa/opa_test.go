// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build opa_wasm
// +build opa_wasm

package opa_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/compile"
	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa"
	wasm_util "github.com/open-policy-agent/opa/internal/wasm/util"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/util"
)

// control dumping in this file
const dump = false

func TestOPA(t *testing.T) {
	type Eval struct {
		NewPolicy string
		NewData   string
		Input     string
		Result    string
	}

	largeInput := `"` + strings.Repeat("a", 2*wasm_util.PageSize) + `"`

	tests := []struct {
		Description string
		Policy      string
		Query       string
		Data        string
		Evals       []Eval
		WantErr     string   // "" (or unset) means no error expected
		Memory      []uint32 // min, max; in pages
	}{
		{
			Description: "No input, no data, static policy",
			Policy:      `a = true`,
			Query:       "data.p.a = x",
			Evals: []Eval{
				{Result: `{{"x": true}}`},
				{Result: `{{"x": true}}`},
			},
		},
		{
			Description: `cryptohmacmd5/crypto.hmac.md5_unicode`,
			Query:       `data.test.p = x`,
			Policy: `package test
			p[mac] {
  			mac := crypto.hmac.md5(input.message, input.key)
			}`,
			Data: ``,
			Evals: []Eval{
				{Input: `{"message": "åäöçß🥲♙Ω", "key": "秘密の"}`, Result: `{"x": ["20a8743c2157ac60b7e8b79c83651b8d"]}`},
			},
		},
		{Description: `cryptohmacmd5/crypto.hmac.md5_unicode`, Query: `data.test.p = x`, Policy: `package test

  p[mac] {
    mac := crypto.hmac.md5(input.message, input.key)
  }
  `, Data: ``, Evals: []Eval{{Input: `{"message": "åäöçß🥲♙Ω", "key": "秘密の"}`, Result: `{{"x":["20a8743c2157ac60b7e8b79c83651b8d"]}}`}}}, {Description: `cryptohmacsha1/crypto.hmac.sha1_unicode`, Query: `data.test.p = x`, Policy: `package test
  
  p[mac] {
    mac := crypto.hmac.sha1(input.message, input.key)
  }
  `, Data: ``, Evals: []Eval{{Input: `{"message": "åäöçß🥲♙Ω", "key": "秘密の"}`, Result: `{{"x":["81759c39013935fcf0de833d44c8018d7c1455dd"]}}`}}}, {Description: `cryptohmacsha256/crypto.hmac.sha256_unicode`, Query: `data.test.p = x`, Policy: `package test
  
  p[mac] {
    mac := crypto.hmac.sha256(input.message, input.key)
  }
  `, Data: ``, Evals: []Eval{{Input: `{"message": "åäöçß🥲♙Ω", "key": "秘密の"}`, Result: `{{"x":["eb90daeb76d4b2571fbdaf94bbb240809faa8fed93ec0c260dd38c3fdf8d963a"]}}`}}}, {Description: `cryptohmacsha512/crypto.hmac.sha512_unicode`, Query: `data.test.p = x`, Policy: `package test
  
  p[mac] {
    mac := crypto.hmac.sha512(input.message, input.key)
  }
  `, Data: ``, Evals: []Eval{{Input: `{"message": "åäöçß🥲♙Ω", "key": "秘密の"}`, Result: `{{"x":["192f5afded233d6e21427aa26ed267ac118cfa2971013d91cbed530c0b208d78138b83dfe1d6cc3553d7bd518f22a481402c723028e1279d1ffbe8f11ea6b125"]}}`}}}, {Description: `graphql_parse_query/success-encoding multibyte characters are supported`, Query: `data.test.p = x`, Policy: strings.Replace(`package test
  gql := [["]]
  # This comment has a ਊ multi-byte character.
  { field(arg: "Has a ਊ multi-byte character.") }
  [["]]
  ast := {"Operations": [{"Name": "", "Operation": "query", "SelectionSet": [{"Alias": "field", "Arguments": [{"Name": "arg", "Value": {"Kind": 3, "Raw": "Has a ਊ multi-byte character."}}], "Name": "field"}]}]}
  p {
      graphql.parse_query(gql) == ast
  }
  `, "[[\"]]", "`", -1), Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":true}}`}}}, {Description: `jwtdecodeverify/es256-unconstrained`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiAiRVMyNTYiLCAidHlwIjogIkpXVCJ9.eyJpc3MiOiAieHh4In0.JvbTLBF06FR70gb7lCbx_ojhp4bk9--B_aULgNlYM0fYf9OSawaqBQp2lwW6FADFtRJ2WFUk5g0zwVOUlnrlzw", {"cert": "-----BEGIN CERTIFICATE-----\nMIIBcDCCARagAwIBAgIJAMZmuGSIfvgzMAoGCCqGSM49BAMCMBMxETAPBgNVBAMM\nCHdoYXRldmVyMB4XDTE4MDgxMDE0Mjg1NFoXDTE4MDkwOTE0Mjg1NFowEzERMA8G\nA1UEAwwId2hhdGV2ZXIwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAATPwn3WCEXL\nmjp/bFniDwuwsfu7bASlPae2PyWhqGeWwe23Xlyx+tSqxlkXYe4pZ23BkAAscpGj\nyn5gXHExyDlKo1MwUTAdBgNVHQ4EFgQUElRjSoVgKjUqY5AXz2o74cLzzS8wHwYD\nVR0jBBgwFoAUElRjSoVgKjUqY5AXz2o74cLzzS8wDwYDVR0TAQH/BAUwAwEB/zAK\nBggqhkjOPQQDAgNIADBFAiEA4yQ/88ZrUX68c6kOe9G11u8NUaUzd8pLOtkKhniN\nOHoCIHmNX37JOqTcTzGn2u9+c8NlnvZ0uDvsd1BmKPaUmjmm\n-----END CERTIFICATE-----\n"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[true,{"alg":"ES256","typ":"JWT"},{"iss":"xxx"}]}}`}}}, {Description: `jwtdecodeverify/hs256-key-wrong`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyIjoiYWxpY2UiLCJhenAiOiJhbGljZSIsInN1Ym9yZGluYXRlcyI6W10sImhyIjpmYWxzZX0.rz3jTY033z-NrKfwrK89_dcLF7TN4gwCMj-fVBDyLoM", {"secret": "the wrong key"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[false,{},{}]}}`}}}, {Description: `jwtdecodeverify/hs256-unconstrained`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyIjoiYWxpY2UiLCJhenAiOiJhbGljZSIsInN1Ym9yZGluYXRlcyI6W10sImhyIjpmYWxzZX0.rz3jTY033z-NrKfwrK89_dcLF7TN4gwCMj-fVBDyLoM", {"secret": "secret"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[true,{"alg":"HS256","typ":"JWT"},{"azp":"alice","hr":false,"subordinates":[],"user":"alice"}]}}`}}}, {Description: `jwtdecodeverify/ps256-alg-ok`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiAiUFMyNTYiLCAidHlwIjogIkpXVCJ9.eyJpc3MiOiAieHh4In0.iCePYnD1U13oBe_6ylhmojmkY_VZNYXqVszAej8RImMGv51OEqARmYFkRZYTiYCiVFober7vcDq_stOj1uAJCuttygGW_dpHiN-3EWsU2E2vCnXlygWe0ud38pOC-OVyEFbXxO9-m51vnS-3VmBjEO8G1UE8bLFXTeFOGkUIj9dqlefJSWh5wa8XA3g9mj0jqpuJi-7QgEIeVHk-JzhGpoFqI2f-Df_agVvc2x4V-6fJmj7wV2IsaFPRi36mVQmg8S-dkxu4AlaeCILhyNZl8ewjBHHBjJFRwzcy88L00mzdO51ZxEYsBdQav3ux2sc6vjT9PvvjAwzcthQxEoEaNA", {"alg": "PS256", "cert": "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIJAJRvYDU3ei3EMA0GCSqGSIb3DQEBCwUAMBMxETAPBgNV\nBAMMCHdoYXRldmVyMB4XDTE4MDgxMDEwMzgxNloXDTE4MDkwOTEwMzgxNlowEzER\nMA8GA1UEAwwId2hhdGV2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQC4kCmzLMW/5jzkzkmN7Me8wPD+ymBUIjsGqliGfMrfFfDV2eTPVtZcYD3IXoB4\nAOUT7XJzWjOsBRFOcVKKEiCPjXiLcwLb/QWQ1x0Budft32r3+N0KQd1rgcRHTPNc\nJoeWCfOgDPp51RTzTT6HQuV4ud+CDhRJP7QMVMIgal9Nuzs49LLZaBPW8/rFsHjk\nJQ4kDujSrpcT6F2FZY3SmWsOJgP7RjVKk5BheYeFKav5ZV4p6iHn/TN4RVpvpNBh\n5z/XoHITJ6lpkHSDpbIaQUTpobU2um8N3biz+HsEAmD9Laa27WUpYSpiM6DDMSXl\ndBDJdumerVRJvXYCtfXqtl17AgMBAAGjUzBRMB0GA1UdDgQWBBRz74MkVzT2K52/\nFJC4mTa9coM/DTAfBgNVHSMEGDAWgBRz74MkVzT2K52/FJC4mTa9coM/DTAPBgNV\nHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAD1ZE4IaIAetqGG+vt9oz1\nIx0j4EPok0ONyhhmiSsF6rSv8zlNWweVf5y6Z+AoTNY1Fym0T7dbpbqIox0EdKV3\nFLzniWOjznupbnqfXwHX/g1UAZSyt3akSatVhvNpGlnd7efTIAiNinX/TkzIjhZ7\nihMIZCGykT1P0ys1OaeEf57wAzviatD4pEMTIW0OOqY8bdRGhuJR1kKUZ/2Nm8Ln\ny7E0y8uODVbH9cAwGyzWB/QFc+bffNgi9uJaPQQc5Zxwpu9utlqyzFvXgV7MBYUK\nEYSLyxp4g4e5aujtLugaC8H6n9vP1mEBr/+T8HGynBZHNTKlDhhL9qDbpkkNB6/w\n-----END CERTIFICATE-----"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[true,{"alg":"PS256","typ":"JWT"},{"iss":"xxx"}]}}`}}}, {Description: `jwtdecodeverify/ps256-alg-wrong`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiAiUFMyNTYiLCAidHlwIjogIkpXVCJ9.eyJpc3MiOiAieHh4In0.iCePYnD1U13oBe_6ylhmojmkY_VZNYXqVszAej8RImMGv51OEqARmYFkRZYTiYCiVFober7vcDq_stOj1uAJCuttygGW_dpHiN-3EWsU2E2vCnXlygWe0ud38pOC-OVyEFbXxO9-m51vnS-3VmBjEO8G1UE8bLFXTeFOGkUIj9dqlefJSWh5wa8XA3g9mj0jqpuJi-7QgEIeVHk-JzhGpoFqI2f-Df_agVvc2x4V-6fJmj7wV2IsaFPRi36mVQmg8S-dkxu4AlaeCILhyNZl8ewjBHHBjJFRwzcy88L00mzdO51ZxEYsBdQav3ux2sc6vjT9PvvjAwzcthQxEoEaNA", {"alg": "RS256", "cert": "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIJAJRvYDU3ei3EMA0GCSqGSIb3DQEBCwUAMBMxETAPBgNV\nBAMMCHdoYXRldmVyMB4XDTE4MDgxMDEwMzgxNloXDTE4MDkwOTEwMzgxNlowEzER\nMA8GA1UEAwwId2hhdGV2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQC4kCmzLMW/5jzkzkmN7Me8wPD+ymBUIjsGqliGfMrfFfDV2eTPVtZcYD3IXoB4\nAOUT7XJzWjOsBRFOcVKKEiCPjXiLcwLb/QWQ1x0Budft32r3+N0KQd1rgcRHTPNc\nJoeWCfOgDPp51RTzTT6HQuV4ud+CDhRJP7QMVMIgal9Nuzs49LLZaBPW8/rFsHjk\nJQ4kDujSrpcT6F2FZY3SmWsOJgP7RjVKk5BheYeFKav5ZV4p6iHn/TN4RVpvpNBh\n5z/XoHITJ6lpkHSDpbIaQUTpobU2um8N3biz+HsEAmD9Laa27WUpYSpiM6DDMSXl\ndBDJdumerVRJvXYCtfXqtl17AgMBAAGjUzBRMB0GA1UdDgQWBBRz74MkVzT2K52/\nFJC4mTa9coM/DTAfBgNVHSMEGDAWgBRz74MkVzT2K52/FJC4mTa9coM/DTAPBgNV\nHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAD1ZE4IaIAetqGG+vt9oz1\nIx0j4EPok0ONyhhmiSsF6rSv8zlNWweVf5y6Z+AoTNY1Fym0T7dbpbqIox0EdKV3\nFLzniWOjznupbnqfXwHX/g1UAZSyt3akSatVhvNpGlnd7efTIAiNinX/TkzIjhZ7\nihMIZCGykT1P0ys1OaeEf57wAzviatD4pEMTIW0OOqY8bdRGhuJR1kKUZ/2Nm8Ln\ny7E0y8uODVbH9cAwGyzWB/QFc+bffNgi9uJaPQQc5Zxwpu9utlqyzFvXgV7MBYUK\nEYSLyxp4g4e5aujtLugaC8H6n9vP1mEBr/+T8HGynBZHNTKlDhhL9qDbpkkNB6/w\n-----END CERTIFICATE-----"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[false,{},{}]}}`}}}, {Description: `jwtdecodeverify/ps256-iss-ok`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiAiUFMyNTYiLCAidHlwIjogIkpXVCJ9.eyJpc3MiOiAieHh4In0.iCePYnD1U13oBe_6ylhmojmkY_VZNYXqVszAej8RImMGv51OEqARmYFkRZYTiYCiVFober7vcDq_stOj1uAJCuttygGW_dpHiN-3EWsU2E2vCnXlygWe0ud38pOC-OVyEFbXxO9-m51vnS-3VmBjEO8G1UE8bLFXTeFOGkUIj9dqlefJSWh5wa8XA3g9mj0jqpuJi-7QgEIeVHk-JzhGpoFqI2f-Df_agVvc2x4V-6fJmj7wV2IsaFPRi36mVQmg8S-dkxu4AlaeCILhyNZl8ewjBHHBjJFRwzcy88L00mzdO51ZxEYsBdQav3ux2sc6vjT9PvvjAwzcthQxEoEaNA", {"cert": "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIJAJRvYDU3ei3EMA0GCSqGSIb3DQEBCwUAMBMxETAPBgNV\nBAMMCHdoYXRldmVyMB4XDTE4MDgxMDEwMzgxNloXDTE4MDkwOTEwMzgxNlowEzER\nMA8GA1UEAwwId2hhdGV2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQC4kCmzLMW/5jzkzkmN7Me8wPD+ymBUIjsGqliGfMrfFfDV2eTPVtZcYD3IXoB4\nAOUT7XJzWjOsBRFOcVKKEiCPjXiLcwLb/QWQ1x0Budft32r3+N0KQd1rgcRHTPNc\nJoeWCfOgDPp51RTzTT6HQuV4ud+CDhRJP7QMVMIgal9Nuzs49LLZaBPW8/rFsHjk\nJQ4kDujSrpcT6F2FZY3SmWsOJgP7RjVKk5BheYeFKav5ZV4p6iHn/TN4RVpvpNBh\n5z/XoHITJ6lpkHSDpbIaQUTpobU2um8N3biz+HsEAmD9Laa27WUpYSpiM6DDMSXl\ndBDJdumerVRJvXYCtfXqtl17AgMBAAGjUzBRMB0GA1UdDgQWBBRz74MkVzT2K52/\nFJC4mTa9coM/DTAfBgNVHSMEGDAWgBRz74MkVzT2K52/FJC4mTa9coM/DTAPBgNV\nHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAD1ZE4IaIAetqGG+vt9oz1\nIx0j4EPok0ONyhhmiSsF6rSv8zlNWweVf5y6Z+AoTNY1Fym0T7dbpbqIox0EdKV3\nFLzniWOjznupbnqfXwHX/g1UAZSyt3akSatVhvNpGlnd7efTIAiNinX/TkzIjhZ7\nihMIZCGykT1P0ys1OaeEf57wAzviatD4pEMTIW0OOqY8bdRGhuJR1kKUZ/2Nm8Ln\ny7E0y8uODVbH9cAwGyzWB/QFc+bffNgi9uJaPQQc5Zxwpu9utlqyzFvXgV7MBYUK\nEYSLyxp4g4e5aujtLugaC8H6n9vP1mEBr/+T8HGynBZHNTKlDhhL9qDbpkkNB6/w\n-----END CERTIFICATE-----", "iss": "xxx"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[true,{"alg":"PS256","typ":"JWT"},{"iss":"xxx"}]}}`}}}, {Description: `jwtdecodeverify/ps256-iss-wrong`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiAiUFMyNTYiLCAidHlwIjogIkpXVCJ9.eyJpc3MiOiAieHh4In0.iCePYnD1U13oBe_6ylhmojmkY_VZNYXqVszAej8RImMGv51OEqARmYFkRZYTiYCiVFober7vcDq_stOj1uAJCuttygGW_dpHiN-3EWsU2E2vCnXlygWe0ud38pOC-OVyEFbXxO9-m51vnS-3VmBjEO8G1UE8bLFXTeFOGkUIj9dqlefJSWh5wa8XA3g9mj0jqpuJi-7QgEIeVHk-JzhGpoFqI2f-Df_agVvc2x4V-6fJmj7wV2IsaFPRi36mVQmg8S-dkxu4AlaeCILhyNZl8ewjBHHBjJFRwzcy88L00mzdO51ZxEYsBdQav3ux2sc6vjT9PvvjAwzcthQxEoEaNA", {"cert": "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIJAJRvYDU3ei3EMA0GCSqGSIb3DQEBCwUAMBMxETAPBgNV\nBAMMCHdoYXRldmVyMB4XDTE4MDgxMDEwMzgxNloXDTE4MDkwOTEwMzgxNlowEzER\nMA8GA1UEAwwId2hhdGV2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQC4kCmzLMW/5jzkzkmN7Me8wPD+ymBUIjsGqliGfMrfFfDV2eTPVtZcYD3IXoB4\nAOUT7XJzWjOsBRFOcVKKEiCPjXiLcwLb/QWQ1x0Budft32r3+N0KQd1rgcRHTPNc\nJoeWCfOgDPp51RTzTT6HQuV4ud+CDhRJP7QMVMIgal9Nuzs49LLZaBPW8/rFsHjk\nJQ4kDujSrpcT6F2FZY3SmWsOJgP7RjVKk5BheYeFKav5ZV4p6iHn/TN4RVpvpNBh\n5z/XoHITJ6lpkHSDpbIaQUTpobU2um8N3biz+HsEAmD9Laa27WUpYSpiM6DDMSXl\ndBDJdumerVRJvXYCtfXqtl17AgMBAAGjUzBRMB0GA1UdDgQWBBRz74MkVzT2K52/\nFJC4mTa9coM/DTAfBgNVHSMEGDAWgBRz74MkVzT2K52/FJC4mTa9coM/DTAPBgNV\nHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAD1ZE4IaIAetqGG+vt9oz1\nIx0j4EPok0ONyhhmiSsF6rSv8zlNWweVf5y6Z+AoTNY1Fym0T7dbpbqIox0EdKV3\nFLzniWOjznupbnqfXwHX/g1UAZSyt3akSatVhvNpGlnd7efTIAiNinX/TkzIjhZ7\nihMIZCGykT1P0ys1OaeEf57wAzviatD4pEMTIW0OOqY8bdRGhuJR1kKUZ/2Nm8Ln\ny7E0y8uODVbH9cAwGyzWB/QFc+bffNgi9uJaPQQc5Zxwpu9utlqyzFvXgV7MBYUK\nEYSLyxp4g4e5aujtLugaC8H6n9vP1mEBr/+T8HGynBZHNTKlDhhL9qDbpkkNB6/w\n-----END CERTIFICATE-----", "iss": "yyy"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[false,{},{}]}}`}}}, {Description: `jwtdecodeverify/ps256-key-wrong`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiAiUFMyNTYiLCAidHlwIjogIkpXVCJ9.eyJpc3MiOiAieHh4In0.iCePYnD1U13oBe_6ylhmojmkY_VZNYXqVszAej8RImMGv51OEqARmYFkRZYTiYCiVFober7vcDq_stOj1uAJCuttygGW_dpHiN-3EWsU2E2vCnXlygWe0ud38pOC-OVyEFbXxO9-m51vnS-3VmBjEO8G1UE8bLFXTeFOGkUIj9dqlefJSWh5wa8XA3g9mj0jqpuJi-7QgEIeVHk-JzhGpoFqI2f-Df_agVvc2x4V-6fJmj7wV2IsaFPRi36mVQmg8S-dkxu4AlaeCILhyNZl8ewjBHHBjJFRwzcy88L00mzdO51ZxEYsBdQav3ux2sc6vjT9PvvjAwzcthQxEoEaNA", {"cert": "-----BEGIN CERTIFICATE-----\nMIIFiDCCA3ACCQCGV6XsfG/oRTANBgkqhkiG9w0BAQUFADCBhTELMAkGA1UEBhMC\nVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFTATBgNVBAcMDFJlZHdvb2QgQ2l0eTEO\nMAwGA1UECgwFU3R5cmExDDAKBgNVBAsMA0RldjESMBAGA1UEAwwJbG9jYWxob3N0\nMRgwFgYJKoZIhvcNAQkBFglhc2hAc3R5cmEwHhcNMTgwMzA2MDAxNTU5WhcNMTkw\nMzA2MDAxNTU5WjCBhTELMAkGA1UEBhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWEx\nFTATBgNVBAcMDFJlZHdvb2QgQ2l0eTEOMAwGA1UECgwFU3R5cmExDDAKBgNVBAsM\nA0RldjESMBAGA1UEAwwJbG9jYWxob3N0MRgwFgYJKoZIhvcNAQkBFglhc2hAc3R5\ncmEwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDucnAwTRA0zqDQ671L\nKWOVwhjhycFyzyhZUd7vhsnslOBiYM6TYIDXhETfAk2RQoRE/9xF16woMD8FOglc\nlSuhi+GNfFRif6LfArm84ZFj1ZS1MX2logikhXhRJQ7AOHe5+ED0re3KH5lWyqfz\nR6bQuPYwTQSBJy6Tq7T9RiOM29yadCX64OaCEbzEFmHtNlbb5px4zCVvgskg/fpV\nGGCMpAYjGDatbxE5eAloVs1EJuI5RSqWr1JRm6EejxM04BFdfGn1HgWrsKXtlvBa\n00/AC0zXL5n6LK7+L3WbRguVTZcE4Yu70gDwhmM+VsKeT9LKClX003BNj0NJDRB9\ndw9MaWxsXDNHNOWEfbnASXeP7ZRv3D81ftij6P8SL14ZnxyrRty8TAN4ij3wd41l\nastRQCtrJFi+HzO606XOp6HDzBoWT0DGl8Sn2hZ6RLPyBnD04vvvcSGeCVjHGOQ8\nc3OTroK58u5MR/q4T00sTkeeVAxuKoEWKsjIBYYrJTe/a2mEq9yiDGbPNYDnWnQZ\njSUZm+Us23Y2sm/agZ5zKXcEuoecGL6sYCixr/xeB9BPxEiTthH+0M8OY99qpIhz\nSmj41wdgQfzZi/6B8pIr77V/KywYKxJEmzw8Uy48aC/rZ8WsT8QdKwclo1aiNJhx\n79OvGbZFoeHD/w7igpx+ttpF/wIDAQABMA0GCSqGSIb3DQEBBQUAA4ICAQC3wWUs\nfXz+aSfFVz+O3mLFkr65NIgazbGAySgMgMNVuadheIkPL4k21atyflfpx4pg9FGv\n40vWCLMajpvynfz4oqah0BACnpqzQ8Dx6HYkmlXK8fLB+WtPrZBeUEsGPKuJYt4M\nd5TeY3VpNgWOPXmnE4lvxHZqh/8OwmOpjBfC9E3e2eqgwiwOkXnMaZEPgKP6JiWk\nEFaQ9jgMQqJZnNcv6NmiqqsZeI0/NNjBpkmEWQl+wLegVusHiQ0FMBMQ0taEo21r\nzUwHoNJR3h3wgGQiKxKOH1FUKHBV7hEqObLraD/hfG5xYucJfvvAAP1iH0ycPs+9\nhSccrn5/HY1c9AZnW8Kh7atp/wFP+sHjtECWK/lUmXfhASS293hprCpJk2n9pkmR\nziXKJhjwkxlC8NcHuiVfaxdfDa4+1Qta2gK7GEypbvLoEmIt/dsYUsxUg84lwJJ9\nnyC/pfZ5a8wFSf186JeVH4kHd3bnkzlQz460HndOMSJ/Xi1wSfuZlOVupFf8TVKl\np4j28MTLH2Wqx50NssKThdaX6hoCiMqreYa+EVaN1f/cIGQxZSCzdzMCKqdB8lKB\n3Eax+5zsIa/UyPwGxZcyXBRHAlz5ZnkjuRxInyiMkBWWz3IZXjTe6Fq8BNd2UWNc\nw35+2nO5n1LKXgR2+nzhZUOk8TPsi9WUywRluQ==\n-----END CERTIFICATE-----"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[false,{},{}]}}`}}}, {Description: `jwtdecodeverify/ps256-no-aud`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiAiUFMyNTYiLCAidHlwIjogIkpXVCJ9.eyJpc3MiOiAieHh4In0.iCePYnD1U13oBe_6ylhmojmkY_VZNYXqVszAej8RImMGv51OEqARmYFkRZYTiYCiVFober7vcDq_stOj1uAJCuttygGW_dpHiN-3EWsU2E2vCnXlygWe0ud38pOC-OVyEFbXxO9-m51vnS-3VmBjEO8G1UE8bLFXTeFOGkUIj9dqlefJSWh5wa8XA3g9mj0jqpuJi-7QgEIeVHk-JzhGpoFqI2f-Df_agVvc2x4V-6fJmj7wV2IsaFPRi36mVQmg8S-dkxu4AlaeCILhyNZl8ewjBHHBjJFRwzcy88L00mzdO51ZxEYsBdQav3ux2sc6vjT9PvvjAwzcthQxEoEaNA", {"aud": "cath", "cert": "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIJAJRvYDU3ei3EMA0GCSqGSIb3DQEBCwUAMBMxETAPBgNV\nBAMMCHdoYXRldmVyMB4XDTE4MDgxMDEwMzgxNloXDTE4MDkwOTEwMzgxNlowEzER\nMA8GA1UEAwwId2hhdGV2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQC4kCmzLMW/5jzkzkmN7Me8wPD+ymBUIjsGqliGfMrfFfDV2eTPVtZcYD3IXoB4\nAOUT7XJzWjOsBRFOcVKKEiCPjXiLcwLb/QWQ1x0Budft32r3+N0KQd1rgcRHTPNc\nJoeWCfOgDPp51RTzTT6HQuV4ud+CDhRJP7QMVMIgal9Nuzs49LLZaBPW8/rFsHjk\nJQ4kDujSrpcT6F2FZY3SmWsOJgP7RjVKk5BheYeFKav5ZV4p6iHn/TN4RVpvpNBh\n5z/XoHITJ6lpkHSDpbIaQUTpobU2um8N3biz+HsEAmD9Laa27WUpYSpiM6DDMSXl\ndBDJdumerVRJvXYCtfXqtl17AgMBAAGjUzBRMB0GA1UdDgQWBBRz74MkVzT2K52/\nFJC4mTa9coM/DTAfBgNVHSMEGDAWgBRz74MkVzT2K52/FJC4mTa9coM/DTAPBgNV\nHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAD1ZE4IaIAetqGG+vt9oz1\nIx0j4EPok0ONyhhmiSsF6rSv8zlNWweVf5y6Z+AoTNY1Fym0T7dbpbqIox0EdKV3\nFLzniWOjznupbnqfXwHX/g1UAZSyt3akSatVhvNpGlnd7efTIAiNinX/TkzIjhZ7\nihMIZCGykT1P0ys1OaeEf57wAzviatD4pEMTIW0OOqY8bdRGhuJR1kKUZ/2Nm8Ln\ny7E0y8uODVbH9cAwGyzWB/QFc+bffNgi9uJaPQQc5Zxwpu9utlqyzFvXgV7MBYUK\nEYSLyxp4g4e5aujtLugaC8H6n9vP1mEBr/+T8HGynBZHNTKlDhhL9qDbpkkNB6/w\n-----END CERTIFICATE-----"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[false,{},{}]}}`}}}, {Description: `jwtdecodeverify/ps256-unconstrained`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiAiUFMyNTYiLCAidHlwIjogIkpXVCJ9.eyJpc3MiOiAieHh4In0.iCePYnD1U13oBe_6ylhmojmkY_VZNYXqVszAej8RImMGv51OEqARmYFkRZYTiYCiVFober7vcDq_stOj1uAJCuttygGW_dpHiN-3EWsU2E2vCnXlygWe0ud38pOC-OVyEFbXxO9-m51vnS-3VmBjEO8G1UE8bLFXTeFOGkUIj9dqlefJSWh5wa8XA3g9mj0jqpuJi-7QgEIeVHk-JzhGpoFqI2f-Df_agVvc2x4V-6fJmj7wV2IsaFPRi36mVQmg8S-dkxu4AlaeCILhyNZl8ewjBHHBjJFRwzcy88L00mzdO51ZxEYsBdQav3ux2sc6vjT9PvvjAwzcthQxEoEaNA", {"cert": "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIJAJRvYDU3ei3EMA0GCSqGSIb3DQEBCwUAMBMxETAPBgNV\nBAMMCHdoYXRldmVyMB4XDTE4MDgxMDEwMzgxNloXDTE4MDkwOTEwMzgxNlowEzER\nMA8GA1UEAwwId2hhdGV2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQC4kCmzLMW/5jzkzkmN7Me8wPD+ymBUIjsGqliGfMrfFfDV2eTPVtZcYD3IXoB4\nAOUT7XJzWjOsBRFOcVKKEiCPjXiLcwLb/QWQ1x0Budft32r3+N0KQd1rgcRHTPNc\nJoeWCfOgDPp51RTzTT6HQuV4ud+CDhRJP7QMVMIgal9Nuzs49LLZaBPW8/rFsHjk\nJQ4kDujSrpcT6F2FZY3SmWsOJgP7RjVKk5BheYeFKav5ZV4p6iHn/TN4RVpvpNBh\n5z/XoHITJ6lpkHSDpbIaQUTpobU2um8N3biz+HsEAmD9Laa27WUpYSpiM6DDMSXl\ndBDJdumerVRJvXYCtfXqtl17AgMBAAGjUzBRMB0GA1UdDgQWBBRz74MkVzT2K52/\nFJC4mTa9coM/DTAfBgNVHSMEGDAWgBRz74MkVzT2K52/FJC4mTa9coM/DTAPBgNV\nHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAD1ZE4IaIAetqGG+vt9oz1\nIx0j4EPok0ONyhhmiSsF6rSv8zlNWweVf5y6Z+AoTNY1Fym0T7dbpbqIox0EdKV3\nFLzniWOjznupbnqfXwHX/g1UAZSyt3akSatVhvNpGlnd7efTIAiNinX/TkzIjhZ7\nihMIZCGykT1P0ys1OaeEf57wAzviatD4pEMTIW0OOqY8bdRGhuJR1kKUZ/2Nm8Ln\ny7E0y8uODVbH9cAwGyzWB/QFc+bffNgi9uJaPQQc5Zxwpu9utlqyzFvXgV7MBYUK\nEYSLyxp4g4e5aujtLugaC8H6n9vP1mEBr/+T8HGynBZHNTKlDhhL9qDbpkkNB6/w\n-----END CERTIFICATE-----"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[true,{"alg":"PS256","typ":"JWT"},{"iss":"xxx"}]}}`}}}, {Description: `jwtdecodeverify/rs256-alg-missing`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJ0eXAiOiAiSldUIiwgImtpZCI6ICJrMSJ9.eyJpc3MiOiAieHh4IiwgInN1YiI6ICJmcmVkIn0.J4J4FgUD_P5fviVVjgvQWJDg-5XYTP_tHCwB3kSlYVKv8vmnZRNh4ke68OxfMP96iM-LZswG2fNqe-_piGIMepF5rCe1iIWAuz3qqkxfS9YVF3hvwoXhjJT0yIgrDMl1lfW5_XipNshZoxddWK3B7dnVW74MFazEEFuefiQm3PdMUX8jWGsmfgPnqBIZTizErNhoIMuRvYaVM1wA2nfrpVGONxMTaw8T0NRwYIuZwubbnNQ1yLhI0y3dsZvQ_lrh9Khtk9fS1V3SRh7aa9AvferJ4T-48qn_V1m3sINPgoA-uLGyyu3k_GkXRYW1yGNC-MH4T2cwhj89WITbIhusgQ", {"cert": "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIJAJRvYDU3ei3EMA0GCSqGSIb3DQEBCwUAMBMxETAPBgNV\nBAMMCHdoYXRldmVyMB4XDTE4MDgxMDEwMzgxNloXDTE4MDkwOTEwMzgxNlowEzER\nMA8GA1UEAwwId2hhdGV2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQC4kCmzLMW/5jzkzkmN7Me8wPD+ymBUIjsGqliGfMrfFfDV2eTPVtZcYD3IXoB4\nAOUT7XJzWjOsBRFOcVKKEiCPjXiLcwLb/QWQ1x0Budft32r3+N0KQd1rgcRHTPNc\nJoeWCfOgDPp51RTzTT6HQuV4ud+CDhRJP7QMVMIgal9Nuzs49LLZaBPW8/rFsHjk\nJQ4kDujSrpcT6F2FZY3SmWsOJgP7RjVKk5BheYeFKav5ZV4p6iHn/TN4RVpvpNBh\n5z/XoHITJ6lpkHSDpbIaQUTpobU2um8N3biz+HsEAmD9Laa27WUpYSpiM6DDMSXl\ndBDJdumerVRJvXYCtfXqtl17AgMBAAGjUzBRMB0GA1UdDgQWBBRz74MkVzT2K52/\nFJC4mTa9coM/DTAfBgNVHSMEGDAWgBRz74MkVzT2K52/FJC4mTa9coM/DTAPBgNV\nHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAD1ZE4IaIAetqGG+vt9oz1\nIx0j4EPok0ONyhhmiSsF6rSv8zlNWweVf5y6Z+AoTNY1Fym0T7dbpbqIox0EdKV3\nFLzniWOjznupbnqfXwHX/g1UAZSyt3akSatVhvNpGlnd7efTIAiNinX/TkzIjhZ7\nihMIZCGykT1P0ys1OaeEf57wAzviatD4pEMTIW0OOqY8bdRGhuJR1kKUZ/2Nm8Ln\ny7E0y8uODVbH9cAwGyzWB/QFc+bffNgi9uJaPQQc5Zxwpu9utlqyzFvXgV7MBYUK\nEYSLyxp4g4e5aujtLugaC8H6n9vP1mEBr/+T8HGynBZHNTKlDhhL9qDbpkkNB6/w\n-----END CERTIFICATE-----"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[false,{},{}]}}`}}}, {Description: `jwtdecodeverify/rs256-aud`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiAiUlMyNTYiLCAidHlwIjogIkpXVCJ9.eyJpc3MiOiAieHh4IiwgImF1ZCI6ICJmcmVkIn0.F-9m2Tx8r1tuQFirazsI4FK05bXX3uP4ut8M2FryJ07k3bQhy262fdwNDmuFcGx0NfL-c80agcwGoTzMWXkVEgZ2KTz0QSAdcdGk3ZWtUy-Mj2IilZ1dzkVvW8LsithYFTGcUtkelFDrJwtMQ0Kum7SXJpC_HCBk4PbftY0XD6jRgHLnQdeT9_J11L4sd19vCdpxxxm3_m_yvUV3ZynzB4vhQbS3CET4EClAVhi-m_gMh9mj85gY1ycIz6-FxWv8xM2Igm2SMeIdyJwAvEGnIauRS928P_OqVCZgCH2Pafnxtzy77Llpxy8XS0xu5PtPw3_azhg33GaXDCFsfz6GpA", {"aud": "fred", "cert": "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIJAJRvYDU3ei3EMA0GCSqGSIb3DQEBCwUAMBMxETAPBgNV\nBAMMCHdoYXRldmVyMB4XDTE4MDgxMDEwMzgxNloXDTE4MDkwOTEwMzgxNlowEzER\nMA8GA1UEAwwId2hhdGV2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQC4kCmzLMW/5jzkzkmN7Me8wPD+ymBUIjsGqliGfMrfFfDV2eTPVtZcYD3IXoB4\nAOUT7XJzWjOsBRFOcVKKEiCPjXiLcwLb/QWQ1x0Budft32r3+N0KQd1rgcRHTPNc\nJoeWCfOgDPp51RTzTT6HQuV4ud+CDhRJP7QMVMIgal9Nuzs49LLZaBPW8/rFsHjk\nJQ4kDujSrpcT6F2FZY3SmWsOJgP7RjVKk5BheYeFKav5ZV4p6iHn/TN4RVpvpNBh\n5z/XoHITJ6lpkHSDpbIaQUTpobU2um8N3biz+HsEAmD9Laa27WUpYSpiM6DDMSXl\ndBDJdumerVRJvXYCtfXqtl17AgMBAAGjUzBRMB0GA1UdDgQWBBRz74MkVzT2K52/\nFJC4mTa9coM/DTAfBgNVHSMEGDAWgBRz74MkVzT2K52/FJC4mTa9coM/DTAPBgNV\nHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAD1ZE4IaIAetqGG+vt9oz1\nIx0j4EPok0ONyhhmiSsF6rSv8zlNWweVf5y6Z+AoTNY1Fym0T7dbpbqIox0EdKV3\nFLzniWOjznupbnqfXwHX/g1UAZSyt3akSatVhvNpGlnd7efTIAiNinX/TkzIjhZ7\nihMIZCGykT1P0ys1OaeEf57wAzviatD4pEMTIW0OOqY8bdRGhuJR1kKUZ/2Nm8Ln\ny7E0y8uODVbH9cAwGyzWB/QFc+bffNgi9uJaPQQc5Zxwpu9utlqyzFvXgV7MBYUK\nEYSLyxp4g4e5aujtLugaC8H6n9vP1mEBr/+T8HGynBZHNTKlDhhL9qDbpkkNB6/w\n-----END CERTIFICATE-----"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[true,{"alg":"RS256","typ":"JWT"},{"aud":"fred","iss":"xxx"}]}}`}}}, {Description: `jwtdecodeverify/rs256-aud-list`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiAiUlMyNTYiLCAidHlwIjogIkpXVCJ9.eyJpc3MiOiAieHh4IiwgImF1ZCI6IFsiZnJlZCIsICJib2IiXX0.k8jW7PUiMkQCKCjnSFBFFKPDO0RXwZgVkLUwUfi8sMdrrcKi12LC8wd5fLBn0YraFtMXWKdMweKf9ZC-K33h5TK7kkTVKOXctF50mleMlUn0Up_XjtdP1v-2WOfivUXcexN1o-hu0kH7sSQnielXIjC2EAleG6A54YUOZFBdzvd1PKHlsxA7x2iiL73uGeFlyxoaMki8E5tx7FY6JGF1RdhWCoIV5A5J8QnwI5EetduJQ505U65Pk7UApWYWu4l2DT7KCCJa5dJaBvCBemVxWaBhCQWtJKU2ZgOEkpiK7b_HsdeRBmpG9Oi1o5mt5ybC09VxSD-lEda_iJO_7i042A", {"aud": "bob", "cert": "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIJAJRvYDU3ei3EMA0GCSqGSIb3DQEBCwUAMBMxETAPBgNV\nBAMMCHdoYXRldmVyMB4XDTE4MDgxMDEwMzgxNloXDTE4MDkwOTEwMzgxNlowEzER\nMA8GA1UEAwwId2hhdGV2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQC4kCmzLMW/5jzkzkmN7Me8wPD+ymBUIjsGqliGfMrfFfDV2eTPVtZcYD3IXoB4\nAOUT7XJzWjOsBRFOcVKKEiCPjXiLcwLb/QWQ1x0Budft32r3+N0KQd1rgcRHTPNc\nJoeWCfOgDPp51RTzTT6HQuV4ud+CDhRJP7QMVMIgal9Nuzs49LLZaBPW8/rFsHjk\nJQ4kDujSrpcT6F2FZY3SmWsOJgP7RjVKk5BheYeFKav5ZV4p6iHn/TN4RVpvpNBh\n5z/XoHITJ6lpkHSDpbIaQUTpobU2um8N3biz+HsEAmD9Laa27WUpYSpiM6DDMSXl\ndBDJdumerVRJvXYCtfXqtl17AgMBAAGjUzBRMB0GA1UdDgQWBBRz74MkVzT2K52/\nFJC4mTa9coM/DTAfBgNVHSMEGDAWgBRz74MkVzT2K52/FJC4mTa9coM/DTAPBgNV\nHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAD1ZE4IaIAetqGG+vt9oz1\nIx0j4EPok0ONyhhmiSsF6rSv8zlNWweVf5y6Z+AoTNY1Fym0T7dbpbqIox0EdKV3\nFLzniWOjznupbnqfXwHX/g1UAZSyt3akSatVhvNpGlnd7efTIAiNinX/TkzIjhZ7\nihMIZCGykT1P0ys1OaeEf57wAzviatD4pEMTIW0OOqY8bdRGhuJR1kKUZ/2Nm8Ln\ny7E0y8uODVbH9cAwGyzWB/QFc+bffNgi9uJaPQQc5Zxwpu9utlqyzFvXgV7MBYUK\nEYSLyxp4g4e5aujtLugaC8H6n9vP1mEBr/+T8HGynBZHNTKlDhhL9qDbpkkNB6/w\n-----END CERTIFICATE-----"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[true,{"alg":"RS256","typ":"JWT"},{"aud":["fred","bob"],"iss":"xxx"}]}}`}}}, {Description: `jwtdecodeverify/rs256-crit-junk`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJjcml0IjogWyJqdW5rIl0sICJraWQiOiAiazEiLCAiYWxnIjogIlJTMjU2IiwgInR5cCI6ICJKV1QiLCAianVuayI6ICJ4eHgifQ.eyJpc3MiOiAieHh4IiwgInN1YiI6ICJmcmVkIn0.YfoUpW5CgDBtxtBuOix3cdYJGT8cX9Mq7wOhIbjDK7eRQUsAmMY_0EQPh7bd7Yi1gLI3e11BKzguf2EHqAa1kbkHWwFniBO-RIi8q42v2uxC4lpEpIjfaaXB5XmsLfAXtYRqh0AObvbSho6VDXBP_Kn81nhIiE2yFbH14_jhRMSxDBs5ToSkXV-XJHw5bONP8NxPqEk9KF3ZJGzN7J_KoD6LjqfYai5K0eLNEIZh4C1WjTdmCKMR4K6ieZRQWZiSsnhSqLSQERir4n22G3QsdY7dOnCp-SS4VYu3V-PfsOSFMvQ-TTAN1geqMZ9A7k1CCLW0wxKBs-KCiYzmRTzwxA", {"cert": "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIJAJRvYDU3ei3EMA0GCSqGSIb3DQEBCwUAMBMxETAPBgNV\nBAMMCHdoYXRldmVyMB4XDTE4MDgxMDEwMzgxNloXDTE4MDkwOTEwMzgxNlowEzER\nMA8GA1UEAwwId2hhdGV2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQC4kCmzLMW/5jzkzkmN7Me8wPD+ymBUIjsGqliGfMrfFfDV2eTPVtZcYD3IXoB4\nAOUT7XJzWjOsBRFOcVKKEiCPjXiLcwLb/QWQ1x0Budft32r3+N0KQd1rgcRHTPNc\nJoeWCfOgDPp51RTzTT6HQuV4ud+CDhRJP7QMVMIgal9Nuzs49LLZaBPW8/rFsHjk\nJQ4kDujSrpcT6F2FZY3SmWsOJgP7RjVKk5BheYeFKav5ZV4p6iHn/TN4RVpvpNBh\n5z/XoHITJ6lpkHSDpbIaQUTpobU2um8N3biz+HsEAmD9Laa27WUpYSpiM6DDMSXl\ndBDJdumerVRJvXYCtfXqtl17AgMBAAGjUzBRMB0GA1UdDgQWBBRz74MkVzT2K52/\nFJC4mTa9coM/DTAfBgNVHSMEGDAWgBRz74MkVzT2K52/FJC4mTa9coM/DTAPBgNV\nHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAD1ZE4IaIAetqGG+vt9oz1\nIx0j4EPok0ONyhhmiSsF6rSv8zlNWweVf5y6Z+AoTNY1Fym0T7dbpbqIox0EdKV3\nFLzniWOjznupbnqfXwHX/g1UAZSyt3akSatVhvNpGlnd7efTIAiNinX/TkzIjhZ7\nihMIZCGykT1P0ys1OaeEf57wAzviatD4pEMTIW0OOqY8bdRGhuJR1kKUZ/2Nm8Ln\ny7E0y8uODVbH9cAwGyzWB/QFc+bffNgi9uJaPQQc5Zxwpu9utlqyzFvXgV7MBYUK\nEYSLyxp4g4e5aujtLugaC8H6n9vP1mEBr/+T8HGynBZHNTKlDhhL9qDbpkkNB6/w\n-----END CERTIFICATE-----"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[false,{},{}]}}`}}}, {Description: `jwtdecodeverify/rs256-exp-now-expired`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiAiUlMyNTYiLCAidHlwIjogIkpXVCJ9.eyJpc3MiOiAieHh4IiwgImV4cCI6IDMwMDB9.hqDP3AzshNhUZMI02U3nLPrj93QFrgs-74XFrF1Vry2bplrz-NKpdVdfTu8iY_bhmkWf2Om5DdwRZj2ZgpGahtnshnHaRq0RyqF-m3Y7oNj6JL_YMwgxsFIIHtBlagBqDU-gZK99iqSOSGqVhvxqX6gCqFgE7vnEGHeeDedtRM53coAJuwzy8rQV9m3TewoofPdPasGv-dBLQZ3qgmnibkSgb7SmFpjXBy8zL3xJXOZhAHYlgcmcEoFVaWlBguIcWA87WZlpCLYcdYTJzSZweC3QLUhZ4RLJW84-LMKp6xWLLPrp3OgnsduB2G9PYMmYw_qCkuY1KGwfH4PvCQbAzQ", {"cert": "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIJAJRvYDU3ei3EMA0GCSqGSIb3DQEBCwUAMBMxETAPBgNV\nBAMMCHdoYXRldmVyMB4XDTE4MDgxMDEwMzgxNloXDTE4MDkwOTEwMzgxNlowEzER\nMA8GA1UEAwwId2hhdGV2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQC4kCmzLMW/5jzkzkmN7Me8wPD+ymBUIjsGqliGfMrfFfDV2eTPVtZcYD3IXoB4\nAOUT7XJzWjOsBRFOcVKKEiCPjXiLcwLb/QWQ1x0Budft32r3+N0KQd1rgcRHTPNc\nJoeWCfOgDPp51RTzTT6HQuV4ud+CDhRJP7QMVMIgal9Nuzs49LLZaBPW8/rFsHjk\nJQ4kDujSrpcT6F2FZY3SmWsOJgP7RjVKk5BheYeFKav5ZV4p6iHn/TN4RVpvpNBh\n5z/XoHITJ6lpkHSDpbIaQUTpobU2um8N3biz+HsEAmD9Laa27WUpYSpiM6DDMSXl\ndBDJdumerVRJvXYCtfXqtl17AgMBAAGjUzBRMB0GA1UdDgQWBBRz74MkVzT2K52/\nFJC4mTa9coM/DTAfBgNVHSMEGDAWgBRz74MkVzT2K52/FJC4mTa9coM/DTAPBgNV\nHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAD1ZE4IaIAetqGG+vt9oz1\nIx0j4EPok0ONyhhmiSsF6rSv8zlNWweVf5y6Z+AoTNY1Fym0T7dbpbqIox0EdKV3\nFLzniWOjznupbnqfXwHX/g1UAZSyt3akSatVhvNpGlnd7efTIAiNinX/TkzIjhZ7\nihMIZCGykT1P0ys1OaeEf57wAzviatD4pEMTIW0OOqY8bdRGhuJR1kKUZ/2Nm8Ln\ny7E0y8uODVbH9cAwGyzWB/QFc+bffNgi9uJaPQQc5Zxwpu9utlqyzFvXgV7MBYUK\nEYSLyxp4g4e5aujtLugaC8H6n9vP1mEBr/+T8HGynBZHNTKlDhhL9qDbpkkNB6/w\n-----END CERTIFICATE-----"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[false,{},{}]}}`}}}, {Description: `jwtdecodeverify/rs256-exp-now-explicit-expired`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    now := time.now_ns()
    io.jwt.decode_verify("eyJhbGciOiAiUlMyNTYiLCAidHlwIjogIkpXVCJ9.eyJpc3MiOiAieHh4IiwgImV4cCI6IDMwMDB9.hqDP3AzshNhUZMI02U3nLPrj93QFrgs-74XFrF1Vry2bplrz-NKpdVdfTu8iY_bhmkWf2Om5DdwRZj2ZgpGahtnshnHaRq0RyqF-m3Y7oNj6JL_YMwgxsFIIHtBlagBqDU-gZK99iqSOSGqVhvxqX6gCqFgE7vnEGHeeDedtRM53coAJuwzy8rQV9m3TewoofPdPasGv-dBLQZ3qgmnibkSgb7SmFpjXBy8zL3xJXOZhAHYlgcmcEoFVaWlBguIcWA87WZlpCLYcdYTJzSZweC3QLUhZ4RLJW84-LMKp6xWLLPrp3OgnsduB2G9PYMmYw_qCkuY1KGwfH4PvCQbAzQ", {"cert": "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIJAJRvYDU3ei3EMA0GCSqGSIb3DQEBCwUAMBMxETAPBgNV\nBAMMCHdoYXRldmVyMB4XDTE4MDgxMDEwMzgxNloXDTE4MDkwOTEwMzgxNlowEzER\nMA8GA1UEAwwId2hhdGV2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQC4kCmzLMW/5jzkzkmN7Me8wPD+ymBUIjsGqliGfMrfFfDV2eTPVtZcYD3IXoB4\nAOUT7XJzWjOsBRFOcVKKEiCPjXiLcwLb/QWQ1x0Budft32r3+N0KQd1rgcRHTPNc\nJoeWCfOgDPp51RTzTT6HQuV4ud+CDhRJP7QMVMIgal9Nuzs49LLZaBPW8/rFsHjk\nJQ4kDujSrpcT6F2FZY3SmWsOJgP7RjVKk5BheYeFKav5ZV4p6iHn/TN4RVpvpNBh\n5z/XoHITJ6lpkHSDpbIaQUTpobU2um8N3biz+HsEAmD9Laa27WUpYSpiM6DDMSXl\ndBDJdumerVRJvXYCtfXqtl17AgMBAAGjUzBRMB0GA1UdDgQWBBRz74MkVzT2K52/\nFJC4mTa9coM/DTAfBgNVHSMEGDAWgBRz74MkVzT2K52/FJC4mTa9coM/DTAPBgNV\nHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAD1ZE4IaIAetqGG+vt9oz1\nIx0j4EPok0ONyhhmiSsF6rSv8zlNWweVf5y6Z+AoTNY1Fym0T7dbpbqIox0EdKV3\nFLzniWOjznupbnqfXwHX/g1UAZSyt3akSatVhvNpGlnd7efTIAiNinX/TkzIjhZ7\nihMIZCGykT1P0ys1OaeEf57wAzviatD4pEMTIW0OOqY8bdRGhuJR1kKUZ/2Nm8Ln\ny7E0y8uODVbH9cAwGyzWB/QFc+bffNgi9uJaPQQc5Zxwpu9utlqyzFvXgV7MBYUK\nEYSLyxp4g4e5aujtLugaC8H6n9vP1mEBr/+T8HGynBZHNTKlDhhL9qDbpkkNB6/w\n-----END CERTIFICATE-----", "time": now}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[false,{},{}]}}`}}}, {Description: `jwtdecodeverify/rs256-key-wrong`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiAiUlMyNTYiLCAidHlwIjogIkpXVCJ9.eyJpc3MiOiAieHh4IiwgImV4cCI6IDMwMDB9.hqDP3AzshNhUZMI02U3nLPrj93QFrgs-74XFrF1Vry2bplrz-NKpdVdfTu8iY_bhmkWf2Om5DdwRZj2ZgpGahtnshnHaRq0RyqF-m3Y7oNj6JL_YMwgxsFIIHtBlagBqDU-gZK99iqSOSGqVhvxqX6gCqFgE7vnEGHeeDedtRM53coAJuwzy8rQV9m3TewoofPdPasGv-dBLQZ3qgmnibkSgb7SmFpjXBy8zL3xJXOZhAHYlgcmcEoFVaWlBguIcWA87WZlpCLYcdYTJzSZweC3QLUhZ4RLJW84-LMKp6xWLLPrp3OgnsduB2G9PYMmYw_qCkuY1KGwfH4PvCQbAzQ", {"cert": "-----BEGIN CERTIFICATE-----\nMIIFiDCCA3ACCQCGV6XsfG/oRTANBgkqhkiG9w0BAQUFADCBhTELMAkGA1UEBhMC\nVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFTATBgNVBAcMDFJlZHdvb2QgQ2l0eTEO\nMAwGA1UECgwFU3R5cmExDDAKBgNVBAsMA0RldjESMBAGA1UEAwwJbG9jYWxob3N0\nMRgwFgYJKoZIhvcNAQkBFglhc2hAc3R5cmEwHhcNMTgwMzA2MDAxNTU5WhcNMTkw\nMzA2MDAxNTU5WjCBhTELMAkGA1UEBhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWEx\nFTATBgNVBAcMDFJlZHdvb2QgQ2l0eTEOMAwGA1UECgwFU3R5cmExDDAKBgNVBAsM\nA0RldjESMBAGA1UEAwwJbG9jYWxob3N0MRgwFgYJKoZIhvcNAQkBFglhc2hAc3R5\ncmEwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDucnAwTRA0zqDQ671L\nKWOVwhjhycFyzyhZUd7vhsnslOBiYM6TYIDXhETfAk2RQoRE/9xF16woMD8FOglc\nlSuhi+GNfFRif6LfArm84ZFj1ZS1MX2logikhXhRJQ7AOHe5+ED0re3KH5lWyqfz\nR6bQuPYwTQSBJy6Tq7T9RiOM29yadCX64OaCEbzEFmHtNlbb5px4zCVvgskg/fpV\nGGCMpAYjGDatbxE5eAloVs1EJuI5RSqWr1JRm6EejxM04BFdfGn1HgWrsKXtlvBa\n00/AC0zXL5n6LK7+L3WbRguVTZcE4Yu70gDwhmM+VsKeT9LKClX003BNj0NJDRB9\ndw9MaWxsXDNHNOWEfbnASXeP7ZRv3D81ftij6P8SL14ZnxyrRty8TAN4ij3wd41l\nastRQCtrJFi+HzO606XOp6HDzBoWT0DGl8Sn2hZ6RLPyBnD04vvvcSGeCVjHGOQ8\nc3OTroK58u5MR/q4T00sTkeeVAxuKoEWKsjIBYYrJTe/a2mEq9yiDGbPNYDnWnQZ\njSUZm+Us23Y2sm/agZ5zKXcEuoecGL6sYCixr/xeB9BPxEiTthH+0M8OY99qpIhz\nSmj41wdgQfzZi/6B8pIr77V/KywYKxJEmzw8Uy48aC/rZ8WsT8QdKwclo1aiNJhx\n79OvGbZFoeHD/w7igpx+ttpF/wIDAQABMA0GCSqGSIb3DQEBBQUAA4ICAQC3wWUs\nfXz+aSfFVz+O3mLFkr65NIgazbGAySgMgMNVuadheIkPL4k21atyflfpx4pg9FGv\n40vWCLMajpvynfz4oqah0BACnpqzQ8Dx6HYkmlXK8fLB+WtPrZBeUEsGPKuJYt4M\nd5TeY3VpNgWOPXmnE4lvxHZqh/8OwmOpjBfC9E3e2eqgwiwOkXnMaZEPgKP6JiWk\nEFaQ9jgMQqJZnNcv6NmiqqsZeI0/NNjBpkmEWQl+wLegVusHiQ0FMBMQ0taEo21r\nzUwHoNJR3h3wgGQiKxKOH1FUKHBV7hEqObLraD/hfG5xYucJfvvAAP1iH0ycPs+9\nhSccrn5/HY1c9AZnW8Kh7atp/wFP+sHjtECWK/lUmXfhASS293hprCpJk2n9pkmR\nziXKJhjwkxlC8NcHuiVfaxdfDa4+1Qta2gK7GEypbvLoEmIt/dsYUsxUg84lwJJ9\nnyC/pfZ5a8wFSf186JeVH4kHd3bnkzlQz460HndOMSJ/Xi1wSfuZlOVupFf8TVKl\np4j28MTLH2Wqx50NssKThdaX6hoCiMqreYa+EVaN1f/cIGQxZSCzdzMCKqdB8lKB\n3Eax+5zsIa/UyPwGxZcyXBRHAlz5ZnkjuRxInyiMkBWWz3IZXjTe6Fq8BNd2UWNc\nw35+2nO5n1LKXgR2+nzhZUOk8TPsi9WUywRluQ==\n-----END CERTIFICATE-----"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[false,{},{}]}}`}}}, {Description: `jwtdecodeverify/rs256-missing-aud`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiAiUlMyNTYiLCAidHlwIjogIkpXVCJ9.eyJpc3MiOiAieHh4IiwgImF1ZCI6ICJmcmVkIn0.F-9m2Tx8r1tuQFirazsI4FK05bXX3uP4ut8M2FryJ07k3bQhy262fdwNDmuFcGx0NfL-c80agcwGoTzMWXkVEgZ2KTz0QSAdcdGk3ZWtUy-Mj2IilZ1dzkVvW8LsithYFTGcUtkelFDrJwtMQ0Kum7SXJpC_HCBk4PbftY0XD6jRgHLnQdeT9_J11L4sd19vCdpxxxm3_m_yvUV3ZynzB4vhQbS3CET4EClAVhi-m_gMh9mj85gY1ycIz6-FxWv8xM2Igm2SMeIdyJwAvEGnIauRS928P_OqVCZgCH2Pafnxtzy77Llpxy8XS0xu5PtPw3_azhg33GaXDCFsfz6GpA", {"cert": "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIJAJRvYDU3ei3EMA0GCSqGSIb3DQEBCwUAMBMxETAPBgNV\nBAMMCHdoYXRldmVyMB4XDTE4MDgxMDEwMzgxNloXDTE4MDkwOTEwMzgxNlowEzER\nMA8GA1UEAwwId2hhdGV2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQC4kCmzLMW/5jzkzkmN7Me8wPD+ymBUIjsGqliGfMrfFfDV2eTPVtZcYD3IXoB4\nAOUT7XJzWjOsBRFOcVKKEiCPjXiLcwLb/QWQ1x0Budft32r3+N0KQd1rgcRHTPNc\nJoeWCfOgDPp51RTzTT6HQuV4ud+CDhRJP7QMVMIgal9Nuzs49LLZaBPW8/rFsHjk\nJQ4kDujSrpcT6F2FZY3SmWsOJgP7RjVKk5BheYeFKav5ZV4p6iHn/TN4RVpvpNBh\n5z/XoHITJ6lpkHSDpbIaQUTpobU2um8N3biz+HsEAmD9Laa27WUpYSpiM6DDMSXl\ndBDJdumerVRJvXYCtfXqtl17AgMBAAGjUzBRMB0GA1UdDgQWBBRz74MkVzT2K52/\nFJC4mTa9coM/DTAfBgNVHSMEGDAWgBRz74MkVzT2K52/FJC4mTa9coM/DTAPBgNV\nHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAD1ZE4IaIAetqGG+vt9oz1\nIx0j4EPok0ONyhhmiSsF6rSv8zlNWweVf5y6Z+AoTNY1Fym0T7dbpbqIox0EdKV3\nFLzniWOjznupbnqfXwHX/g1UAZSyt3akSatVhvNpGlnd7efTIAiNinX/TkzIjhZ7\nihMIZCGykT1P0ys1OaeEf57wAzviatD4pEMTIW0OOqY8bdRGhuJR1kKUZ/2Nm8Ln\ny7E0y8uODVbH9cAwGyzWB/QFc+bffNgi9uJaPQQc5Zxwpu9utlqyzFvXgV7MBYUK\nEYSLyxp4g4e5aujtLugaC8H6n9vP1mEBr/+T8HGynBZHNTKlDhhL9qDbpkkNB6/w\n-----END CERTIFICATE-----"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[false,{},{}]}}`}}}, {Description: `jwtdecodeverify/rs256-nbf-now-ok`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiAiUlMyNTYiLCAidHlwIjogIkpXVCJ9.eyJuYmYiOiAxMDAwLCAiaXNzIjogInh4eCJ9.cwwYDfJhU_ambPIpwBJwDek05miffoudprr41IAYsl0IKekb1ii2uEgwkNM-LJtVXHe9hsK3gANFyfqoJuCZIBvaNMx_3Z0BUdeBs4k1UwBiZCpuud0ofgHKURwvehNgqDvRfchq_-K_Agi2iRdl0oShgLjN-gVbBl8pRwUbQrvASlcsCpZIKUyOzXNtaIZEFh1z6ISDy8UHHOdoieKpN23swya7QAcEb0wXEEKMkkhiRd5QHgWLk37Lnw2K89mKcq4Om0CtV9nHrxxmpYGSMPojCy16Gjdg5-xKyJWvxCfb3YUBUVM4RWa7ICOPRJWPuHxu9pPYG63hb_qDU6NLsw", {"cert": "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIJAJRvYDU3ei3EMA0GCSqGSIb3DQEBCwUAMBMxETAPBgNV\nBAMMCHdoYXRldmVyMB4XDTE4MDgxMDEwMzgxNloXDTE4MDkwOTEwMzgxNlowEzER\nMA8GA1UEAwwId2hhdGV2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQC4kCmzLMW/5jzkzkmN7Me8wPD+ymBUIjsGqliGfMrfFfDV2eTPVtZcYD3IXoB4\nAOUT7XJzWjOsBRFOcVKKEiCPjXiLcwLb/QWQ1x0Budft32r3+N0KQd1rgcRHTPNc\nJoeWCfOgDPp51RTzTT6HQuV4ud+CDhRJP7QMVMIgal9Nuzs49LLZaBPW8/rFsHjk\nJQ4kDujSrpcT6F2FZY3SmWsOJgP7RjVKk5BheYeFKav5ZV4p6iHn/TN4RVpvpNBh\n5z/XoHITJ6lpkHSDpbIaQUTpobU2um8N3biz+HsEAmD9Laa27WUpYSpiM6DDMSXl\ndBDJdumerVRJvXYCtfXqtl17AgMBAAGjUzBRMB0GA1UdDgQWBBRz74MkVzT2K52/\nFJC4mTa9coM/DTAfBgNVHSMEGDAWgBRz74MkVzT2K52/FJC4mTa9coM/DTAPBgNV\nHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAD1ZE4IaIAetqGG+vt9oz1\nIx0j4EPok0ONyhhmiSsF6rSv8zlNWweVf5y6Z+AoTNY1Fym0T7dbpbqIox0EdKV3\nFLzniWOjznupbnqfXwHX/g1UAZSyt3akSatVhvNpGlnd7efTIAiNinX/TkzIjhZ7\nihMIZCGykT1P0ys1OaeEf57wAzviatD4pEMTIW0OOqY8bdRGhuJR1kKUZ/2Nm8Ln\ny7E0y8uODVbH9cAwGyzWB/QFc+bffNgi9uJaPQQc5Zxwpu9utlqyzFvXgV7MBYUK\nEYSLyxp4g4e5aujtLugaC8H6n9vP1mEBr/+T8HGynBZHNTKlDhhL9qDbpkkNB6/w\n-----END CERTIFICATE-----"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[true,{"alg":"RS256","typ":"JWT"},{"iss":"xxx","nbf":1000}]}}`}}}, {Description: `jwtdecodeverify/rs256-wrong-aud`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiAiUlMyNTYiLCAidHlwIjogIkpXVCJ9.eyJpc3MiOiAieHh4IiwgImF1ZCI6ICJmcmVkIn0.F-9m2Tx8r1tuQFirazsI4FK05bXX3uP4ut8M2FryJ07k3bQhy262fdwNDmuFcGx0NfL-c80agcwGoTzMWXkVEgZ2KTz0QSAdcdGk3ZWtUy-Mj2IilZ1dzkVvW8LsithYFTGcUtkelFDrJwtMQ0Kum7SXJpC_HCBk4PbftY0XD6jRgHLnQdeT9_J11L4sd19vCdpxxxm3_m_yvUV3ZynzB4vhQbS3CET4EClAVhi-m_gMh9mj85gY1ycIz6-FxWv8xM2Igm2SMeIdyJwAvEGnIauRS928P_OqVCZgCH2Pafnxtzy77Llpxy8XS0xu5PtPw3_azhg33GaXDCFsfz6GpA", {"aud": "cath", "cert": "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIJAJRvYDU3ei3EMA0GCSqGSIb3DQEBCwUAMBMxETAPBgNV\nBAMMCHdoYXRldmVyMB4XDTE4MDgxMDEwMzgxNloXDTE4MDkwOTEwMzgxNlowEzER\nMA8GA1UEAwwId2hhdGV2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQC4kCmzLMW/5jzkzkmN7Me8wPD+ymBUIjsGqliGfMrfFfDV2eTPVtZcYD3IXoB4\nAOUT7XJzWjOsBRFOcVKKEiCPjXiLcwLb/QWQ1x0Budft32r3+N0KQd1rgcRHTPNc\nJoeWCfOgDPp51RTzTT6HQuV4ud+CDhRJP7QMVMIgal9Nuzs49LLZaBPW8/rFsHjk\nJQ4kDujSrpcT6F2FZY3SmWsOJgP7RjVKk5BheYeFKav5ZV4p6iHn/TN4RVpvpNBh\n5z/XoHITJ6lpkHSDpbIaQUTpobU2um8N3biz+HsEAmD9Laa27WUpYSpiM6DDMSXl\ndBDJdumerVRJvXYCtfXqtl17AgMBAAGjUzBRMB0GA1UdDgQWBBRz74MkVzT2K52/\nFJC4mTa9coM/DTAfBgNVHSMEGDAWgBRz74MkVzT2K52/FJC4mTa9coM/DTAPBgNV\nHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAD1ZE4IaIAetqGG+vt9oz1\nIx0j4EPok0ONyhhmiSsF6rSv8zlNWweVf5y6Z+AoTNY1Fym0T7dbpbqIox0EdKV3\nFLzniWOjznupbnqfXwHX/g1UAZSyt3akSatVhvNpGlnd7efTIAiNinX/TkzIjhZ7\nihMIZCGykT1P0ys1OaeEf57wAzviatD4pEMTIW0OOqY8bdRGhuJR1kKUZ/2Nm8Ln\ny7E0y8uODVbH9cAwGyzWB/QFc+bffNgi9uJaPQQc5Zxwpu9utlqyzFvXgV7MBYUK\nEYSLyxp4g4e5aujtLugaC8H6n9vP1mEBr/+T8HGynBZHNTKlDhhL9qDbpkkNB6/w\n-----END CERTIFICATE-----"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[false,{},{}]}}`}}}, {Description: `jwtdecodeverify/rs256-wrong-aud-list`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiAiUlMyNTYiLCAidHlwIjogIkpXVCJ9.eyJpc3MiOiAieHh4IiwgImF1ZCI6IFsiZnJlZCIsICJib2IiXX0.k8jW7PUiMkQCKCjnSFBFFKPDO0RXwZgVkLUwUfi8sMdrrcKi12LC8wd5fLBn0YraFtMXWKdMweKf9ZC-K33h5TK7kkTVKOXctF50mleMlUn0Up_XjtdP1v-2WOfivUXcexN1o-hu0kH7sSQnielXIjC2EAleG6A54YUOZFBdzvd1PKHlsxA7x2iiL73uGeFlyxoaMki8E5tx7FY6JGF1RdhWCoIV5A5J8QnwI5EetduJQ505U65Pk7UApWYWu4l2DT7KCCJa5dJaBvCBemVxWaBhCQWtJKU2ZgOEkpiK7b_HsdeRBmpG9Oi1o5mt5ybC09VxSD-lEda_iJO_7i042A", {"aud": "cath", "cert": "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIJAJRvYDU3ei3EMA0GCSqGSIb3DQEBCwUAMBMxETAPBgNV\nBAMMCHdoYXRldmVyMB4XDTE4MDgxMDEwMzgxNloXDTE4MDkwOTEwMzgxNlowEzER\nMA8GA1UEAwwId2hhdGV2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQC4kCmzLMW/5jzkzkmN7Me8wPD+ymBUIjsGqliGfMrfFfDV2eTPVtZcYD3IXoB4\nAOUT7XJzWjOsBRFOcVKKEiCPjXiLcwLb/QWQ1x0Budft32r3+N0KQd1rgcRHTPNc\nJoeWCfOgDPp51RTzTT6HQuV4ud+CDhRJP7QMVMIgal9Nuzs49LLZaBPW8/rFsHjk\nJQ4kDujSrpcT6F2FZY3SmWsOJgP7RjVKk5BheYeFKav5ZV4p6iHn/TN4RVpvpNBh\n5z/XoHITJ6lpkHSDpbIaQUTpobU2um8N3biz+HsEAmD9Laa27WUpYSpiM6DDMSXl\ndBDJdumerVRJvXYCtfXqtl17AgMBAAGjUzBRMB0GA1UdDgQWBBRz74MkVzT2K52/\nFJC4mTa9coM/DTAfBgNVHSMEGDAWgBRz74MkVzT2K52/FJC4mTa9coM/DTAPBgNV\nHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAD1ZE4IaIAetqGG+vt9oz1\nIx0j4EPok0ONyhhmiSsF6rSv8zlNWweVf5y6Z+AoTNY1Fym0T7dbpbqIox0EdKV3\nFLzniWOjznupbnqfXwHX/g1UAZSyt3akSatVhvNpGlnd7efTIAiNinX/TkzIjhZ7\nihMIZCGykT1P0ys1OaeEf57wAzviatD4pEMTIW0OOqY8bdRGhuJR1kKUZ/2Nm8Ln\ny7E0y8uODVbH9cAwGyzWB/QFc+bffNgi9uJaPQQc5Zxwpu9utlqyzFvXgV7MBYUK\nEYSLyxp4g4e5aujtLugaC8H6n9vP1mEBr/+T8HGynBZHNTKlDhhL9qDbpkkNB6/w\n-----END CERTIFICATE-----"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[false,{},{}]}}`}}}, {Description: `jwtdecodeverify/rsa256-nested`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiAiUlMyNTYiLCAidHlwIjogIkpXVCIsICJjdHkiOiAiSldUIn0.ZXlKaGJHY2lPaUFpVWxNeU5UWWlMQ0FpZEhsd0lqb2dJa3BYVkNKOS5leUpwYzNNaU9pQWllSGg0SW4wLnJSUnJlUU9DYW9ZLW1Nazcyak5GZVk1YVlFUWhJZ0lFdFZkUTlYblltUUwyTHdfaDdNbkk0U0VPMVBwa0JIVEpyZnljbEplTHpfalJ2UGdJMlcxaDFCNGNaVDhDZ21pVXdxQXI5c0puZHlVQ1FtSWRrbm53WkI5cXAtX3BTdGRHWEo5WnAzeEo4NXotVEJpWlN0QUNUZFdlUklGSUU3VkxPa20tRmxZdzh5OTdnaUN4TmxUdWl3amxlTjMwZDhnWHUxNkZGQzJTSlhtRjZKbXYtNjJHbERhLW1CWFZ0bGJVSTVlWVUwaTdueTNyQjBYUVQxRkt4ZUZ3OF85N09FdV9jY3VLcl82ZHlHZVFHdnQ5Y3JJeEFBMWFZbDdmbVBrNkVhcjllTTNKaGVYMi00Wkx0d1FOY1RDT01YV0dIck1DaG5MWVc4WEFrTHJEbl9yRmxUaVMtZw.Xicc2sWCZ_Nithucsw9XD7YOKrirUdEnH3MyiPM-Ck3vEU2RsTBsfU2JPhfjp3phc0VOgsAXCzwU5PwyNyUo1490q8YSym-liMyO2Lk-hjH5fAxoizg9yD4II_lK6Wz_Tnpc0bBGDLdbuUhvgvO7yqo-leBQlsfRXOvw4VSPSEy8QPtbURtbnLpWY2jGBKz7vGI_o4qDJ3PicG0kyEiWZNh3wjeeCYRCWvXN8qh7Uk5EA-8J5vX651GqV-7gmaX1n-8DXamhaCQcE-p1cjSj04-X-_bJlQtmb-TT3bSyUPxgHVncvxNUby8jkUTzfi5MMbmIzWWkxI5YtJTdtmCkPQ", {"cert": "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIJAJRvYDU3ei3EMA0GCSqGSIb3DQEBCwUAMBMxETAPBgNV\nBAMMCHdoYXRldmVyMB4XDTE4MDgxMDEwMzgxNloXDTE4MDkwOTEwMzgxNlowEzER\nMA8GA1UEAwwId2hhdGV2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQC4kCmzLMW/5jzkzkmN7Me8wPD+ymBUIjsGqliGfMrfFfDV2eTPVtZcYD3IXoB4\nAOUT7XJzWjOsBRFOcVKKEiCPjXiLcwLb/QWQ1x0Budft32r3+N0KQd1rgcRHTPNc\nJoeWCfOgDPp51RTzTT6HQuV4ud+CDhRJP7QMVMIgal9Nuzs49LLZaBPW8/rFsHjk\nJQ4kDujSrpcT6F2FZY3SmWsOJgP7RjVKk5BheYeFKav5ZV4p6iHn/TN4RVpvpNBh\n5z/XoHITJ6lpkHSDpbIaQUTpobU2um8N3biz+HsEAmD9Laa27WUpYSpiM6DDMSXl\ndBDJdumerVRJvXYCtfXqtl17AgMBAAGjUzBRMB0GA1UdDgQWBBRz74MkVzT2K52/\nFJC4mTa9coM/DTAfBgNVHSMEGDAWgBRz74MkVzT2K52/FJC4mTa9coM/DTAPBgNV\nHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAD1ZE4IaIAetqGG+vt9oz1\nIx0j4EPok0ONyhhmiSsF6rSv8zlNWweVf5y6Z+AoTNY1Fym0T7dbpbqIox0EdKV3\nFLzniWOjznupbnqfXwHX/g1UAZSyt3akSatVhvNpGlnd7efTIAiNinX/TkzIjhZ7\nihMIZCGykT1P0ys1OaeEf57wAzviatD4pEMTIW0OOqY8bdRGhuJR1kKUZ/2Nm8Ln\ny7E0y8uODVbH9cAwGyzWB/QFc+bffNgi9uJaPQQc5Zxwpu9utlqyzFvXgV7MBYUK\nEYSLyxp4g4e5aujtLugaC8H6n9vP1mEBr/+T8HGynBZHNTKlDhhL9qDbpkkNB6/w\n-----END CERTIFICATE-----"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[true,{"alg":"RS256","typ":"JWT"},{"iss":"xxx"}]}}`}}}, {Description: `jwtdecodeverify/rsa256-nested2`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y, z] {
    io.jwt.decode_verify("eyJhbGciOiAiUlMyNTYiLCAidHlwIjogIkpXVCIsICJjdHkiOiAiSldUIn0.ZXlKaGJHY2lPaUFpVWxNeU5UWWlMQ0FpZEhsd0lqb2dJa3BYVkNJc0lDSmpkSGtpT2lBaVNsZFVJbjAuWlhsS2FHSkhZMmxQYVVGcFZXeE5lVTVVV1dsTVEwRnBaRWhzZDBscWIyZEphM0JZVmtOS09TNWxlVXB3WXpOTmFVOXBRV2xsU0dnMFNXNHdMbkpTVW5KbFVVOURZVzlaTFcxTmF6Y3lhazVHWlZrMVlWbEZVV2hKWjBsRmRGWmtVVGxZYmxsdFVVd3lUSGRmYURkTmJrazBVMFZQTVZCd2EwSklWRXB5Wm5samJFcGxUSHBmYWxKMlVHZEpNbGN4YURGQ05HTmFWRGhEWjIxcFZYZHhRWEk1YzBwdVpIbFZRMUZ0U1dScmJtNTNXa0k1Y1hBdFgzQlRkR1JIV0VvNVduQXplRW80TlhvdFZFSnBXbE4wUVVOVVpGZGxVa2xHU1VVM1ZreFBhMjB0Um14WmR6aDVPVGRuYVVONFRteFVkV2wzYW14bFRqTXdaRGhuV0hVeE5rWkdRekpUU2xodFJqWktiWFl0TmpKSGJFUmhMVzFDV0ZaMGJHSlZTVFZsV1ZVd2FUZHVlVE55UWpCWVVWUXhSa3Q0WlVaM09GODVOMDlGZFY5alkzVkxjbDgyWkhsSFpWRkhkblE1WTNKSmVFRkJNV0ZaYkRkbWJWQnJOa1ZoY2psbFRUTkthR1ZZTWkwMFdreDBkMUZPWTFSRFQwMVlWMGRJY2sxRGFHNU1XVmM0V0VGclRISkVibDl5Um14VWFWTXRady5YaWNjMnNXQ1pfTml0aHVjc3c5WEQ3WU9LcmlyVWRFbkgzTXlpUE0tQ2szdkVVMlJzVEJzZlUySlBoZmpwM3BoYzBWT2dzQVhDendVNVB3eU55VW8xNDkwcThZU3ltLWxpTXlPMkxrLWhqSDVmQXhvaXpnOXlENElJX2xLNld6X1RucGMwYkJHRExkYnVVaHZndk83eXFvLWxlQlFsc2ZSWE92dzRWU1BTRXk4UVB0YlVSdGJuTHBXWTJqR0JLejd2R0lfbzRxREozUGljRzBreUVpV1pOaDN3amVlQ1lSQ1d2WE44cWg3VWs1RUEtOEo1dlg2NTFHcVYtN2dtYVgxbi04RFhhbWhhQ1FjRS1wMWNqU2owNC1YLV9iSmxRdG1iLVRUM2JTeVVQeGdIVm5jdnhOVWJ5OGprVVR6Zmk1TU1ibUl6V1dreEk1WXRKVGR0bUNrUFE.ODBVH_gooCLJxtPVr1MjJC1syG4MnVUFP9LkI9pSaj0QABV4vpfqrBshHn8zOPgUTDeHwbc01Qy96cQlTMQQb94YANmZyL1nzwmdR4piiGXMGSlcCNfDg1o8DK4msMSR-X-j2IkxBDB8rfeFSfLRMgDCjAF0JolW7qWmMD9tBmFNYAjly4vMwToOXosDmFLl5eqyohXDf-3Ohljm5kIjtyMWkt5S9EVuwlIXh2owK5l59c4-TH29gkuaZ3uU4LFPjD7XKUrlOQnEMuu2QD8LAqTyxbnY4JyzUWEvyTM1dVmGnFpLKCg9QBly__y1u2ffhvDsHyuCmEKAbhPE98YvFA", {"cert": "-----BEGIN CERTIFICATE-----\nMIIC/DCCAeSgAwIBAgIJAJRvYDU3ei3EMA0GCSqGSIb3DQEBCwUAMBMxETAPBgNV\nBAMMCHdoYXRldmVyMB4XDTE4MDgxMDEwMzgxNloXDTE4MDkwOTEwMzgxNlowEzER\nMA8GA1UEAwwId2hhdGV2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB\nAQC4kCmzLMW/5jzkzkmN7Me8wPD+ymBUIjsGqliGfMrfFfDV2eTPVtZcYD3IXoB4\nAOUT7XJzWjOsBRFOcVKKEiCPjXiLcwLb/QWQ1x0Budft32r3+N0KQd1rgcRHTPNc\nJoeWCfOgDPp51RTzTT6HQuV4ud+CDhRJP7QMVMIgal9Nuzs49LLZaBPW8/rFsHjk\nJQ4kDujSrpcT6F2FZY3SmWsOJgP7RjVKk5BheYeFKav5ZV4p6iHn/TN4RVpvpNBh\n5z/XoHITJ6lpkHSDpbIaQUTpobU2um8N3biz+HsEAmD9Laa27WUpYSpiM6DDMSXl\ndBDJdumerVRJvXYCtfXqtl17AgMBAAGjUzBRMB0GA1UdDgQWBBRz74MkVzT2K52/\nFJC4mTa9coM/DTAfBgNVHSMEGDAWgBRz74MkVzT2K52/FJC4mTa9coM/DTAPBgNV\nHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAD1ZE4IaIAetqGG+vt9oz1\nIx0j4EPok0ONyhhmiSsF6rSv8zlNWweVf5y6Z+AoTNY1Fym0T7dbpbqIox0EdKV3\nFLzniWOjznupbnqfXwHX/g1UAZSyt3akSatVhvNpGlnd7efTIAiNinX/TkzIjhZ7\nihMIZCGykT1P0ys1OaeEf57wAzviatD4pEMTIW0OOqY8bdRGhuJR1kKUZ/2Nm8Ln\ny7E0y8uODVbH9cAwGyzWB/QFc+bffNgi9uJaPQQc5Zxwpu9utlqyzFvXgV7MBYUK\nEYSLyxp4g4e5aujtLugaC8H6n9vP1mEBr/+T8HGynBZHNTKlDhhL9qDbpkkNB6/w\n-----END CERTIFICATE-----"}, [x, y, z])
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[true,{"alg":"RS256","typ":"JWT"},{"iss":"xxx"}]}}`}}}, {Description: `reachable_paths/multiple_paths`, Query: `data.reachable.p = x`, Policy: `package reachable
  
  p = result {
    graph.reachable_paths(input.graph, input.initial, result)
  }
  `, Data: `{}`, Evals: []Eval{{Input: `{ "graph": { "a": ["b", "c"], "b": ["c"], "c": [] }, "initial": ["a"] }`, Result: `{{"x":[["a","b","c"],["a","c"]]}}`}}}, {Description: `strings/indexof_n_unicode_matches`, Query: `data.test.p = x`, Policy: `package test
  p := indexof_n("😇😀😇😀😇😀", "😀")
  `, Data: ``, Evals: []Eval{{Input: ``, Result: `{{"x":[1,3,5]}}`}}}, {Description: `withkeyword/invalidate comprehension cache`, Query: `data.generated.p = x`, Policy: `package generated
  
  p = [x, y] {
    x = data.ex.s with input as {"a": "b", "c": "b"}
    y = data.ex.s with input as {"a": "b"}
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":[[{"b":["a"]}],[{"b":["a","c"]}]]}}`}}}, {Description: `withkeyword/with virtual doc exact value`, Query: `data.generated.p = x`, Policy: `package generated
  
  p[x] {
    data.ex.virtual = x with data.a.b as {"c": 1, "d": 2, "e": 1}
  }
  `, Data: `{"a": [1, 2, 3, 4], "b": {"v1": "hello", "v2": "goodbye"}}`, Evals: []Eval{{Input: ``, Result: `{{"x":[["c","e"]]}}`}}}, {Description: `functions/default`, Query: `data.p.m = x`, Policy: `package p.m
  
  default hello = false
  
  hello() = m {
    m = input.message
    1 == 2
    m = "world"
  }
  h = m {
    m = hello()
  }
  `, Data: `null`, Evals: []Eval{{Input: ``, Result: `{{"x":{"hello":false}}}`}}},
		{
			Description: "net.lookup_ip_addr/simple ip4 returns that ip4",
			Policy: `package test
			p = x {
				  x := net.lookup_ip_addr("10.0.0.0")
				  }`,
			Query: "data.test.p = x",
			Evals: []Eval{
				{Result: `{{"x":["10.0.0.0"]}}`},
			},
		},
		{
			Description: "net.lookup_ip_addr/simple ip6 returns that ip6",
			Policy: `package test
	
			p = x {
				  x := net.lookup_ip_addr("::")
				  }
				  `,
			Query: "data.test.p = x",
			Evals: []Eval{
				{Result: `{{"x":["::"]}}`}},
		},
		{
			Description: "net.lookup_ip_addr/localhost",
			Policy: `package test
			p {
				  net.lookup_ip_addr("localhost") == {"127.0.0.1"}
			}
			p {
				  net.lookup_ip_addr("localhost") == {"127.0.0.1", "::1"}
				  }
				  p {
					  net.lookup_ip_addr("localhost") == {"::1"}
					}
				  `,
			Query: "data.test.p = x",
			Evals: []Eval{
				{Result: `{{"x": true}}`},
			},
		},
		{
			Description: "Only input changing",
			Policy:      `a = input`,
			Query:       "data.p.a = x",
			Evals: []Eval{
				{Input: "false", Result: `{{"x": false}}`},
				{Input: "true", Result: `{{"x": true}}`},
			},
		},
		{
			Description: "Only data changing",
			Policy:      `a = data.q`,
			Query:       "data.p.a = x",
			Data:        `{"q": false}`,
			Evals: []Eval{
				{Result: `{{"x": false}}`},
				{NewData: `{"q": true}`, Result: `{{"x": true}}`},
			},
		},
		{
			Description: "Only policy changing",
			Policy:      `a = data.q`,
			Query:       "data.p.a = x",
			Data:        `{"q": false, "r": true}`,
			Evals: []Eval{
				{Result: `{{"x": false}}`},
				{NewPolicy: `a = data.r`, Result: `{{"x": true}}`},
			},
		},
		{
			Description: "Policy and data changing",
			Policy:      `a = data.q`,
			Query:       "data.p.a = x",
			Data:        `{"q": 0, "r": 1}`,
			Evals: []Eval{
				{Result: `{{"x": 0}}`},
				{NewPolicy: `a = data.r`, NewData: `{"q": 2, "r": 3}`, Result: `{{"x": 3}}`},
			},
		},
		{
			Description: "Builtins",
			Policy:      `a = count(data.q) + sum(data.q)`, // builtin not implemented in wasm.
			Query:       "data.p.a = x",
			Evals: []Eval{
				{NewData: `{"q": []}`, Result: `{{"x": 0}}`},
				{NewData: `{"q": [1, 2]}`, Result: `{{"x": 5}}`},
			},
		},
		{
			Description: "Undefined decision",
			Policy:      `a = true`,
			Query:       "data.p.b = x",
			Evals: []Eval{
				{Result: `set()`},
			},
		},
		{
			Description: "jwt data",
			Policy: `a = io.jwt.encode_sign({
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
			})`,
			Query: "data.p.a = x",
			Evals: []Eval{
				{Result: `{{"x": "eyJhbGciOiAiSFMyNTYiLCAidHlwIjogIkpXVCJ9.eyJhdWQiOiBbImJvYiIsICJzYXVsIl0sICJleHAiOiAxMzAwODE5MzgwLCAiaHR0cDovL2V4YW1wbGUuY29tL2lzX3Jvb3QiOiB0cnVlLCAiaXNzIjogImpvZSIsICJwcml2YXRlUGFyYW1zIjogeyJwcml2YXRlX29uZSI6ICJvbmUiLCAicHJpdmF0ZV90d28iOiAidHdvIn19.M10TcaFADr_JYAx7qJ71wktdyuN4IAnhWvVbgrZ5j_4"}}`},
			},
		},
		{
			Description: "jwt data decode",
			Policy:      `q=x{[x, _, _] := io.jwt.decode("eyJhbGciOiAiSFMyNTYiLCAidHlwIjogIkpXVCJ9.eyJhdWQiOiBbImJvYiIsICJzYXVsIl0sICJleHAiOiAxMzAwODE5MzgwLCAiaHR0cDovL2V4YW1wbGUuY29tL2lzX3Jvb3QiOiB0cnVlLCAiaXNzIjogImpvZSIsICJwcml2YXRlUGFyYW1zIjogeyJwcml2YXRlX29uZSI6ICJvbmUiLCAicHJpdmF0ZV90d28iOiAidHdvIn19.M10TcaFADr_JYAx7qJ71wktdyuN4IAnhWvVbgrZ5j_4")}`,
			Query:       "data.p.q = x",
			Evals: []Eval{
				{Result: `{{"x":{"alg":"HS256","typ":"JWT"}}}`},
				{NewPolicy: `q=x{[_, x, _] := io.jwt.decode("eyJhbGciOiAiSFMyNTYiLCAidHlwIjogIkpXVCJ9.eyJhdWQiOiBbImJvYiIsICJzYXVsIl0sICJleHAiOiAxMzAwODE5MzgwLCAiaHR0cDovL2V4YW1wbGUuY29tL2lzX3Jvb3QiOiB0cnVlLCAiaXNzIjogImpvZSIsICJwcml2YXRlUGFyYW1zIjogeyJwcml2YXRlX29uZSI6ICJvbmUiLCAicHJpdmF0ZV90d28iOiAidHdvIn19.M10TcaFADr_JYAx7qJ71wktdyuN4IAnhWvVbgrZ5j_4")}`, Result: `{{"x":{"aud":["bob","saul"],"exp":1300819380,"http://example.com/is_root":true,"iss":"joe","privateParams":{"private_one":"one","private_two":"two"}}}}`},
				{NewPolicy: `q=x{[_, _, x] = io.jwt.decode("eyJhbGciOiAiSFMyNTYiLCAidHlwIjogIkpXVCJ9.eyJhdWQiOiBbImJvYiIsICJzYXVsIl0sICJleHAiOiAxMzAwODE5MzgwLCAiaHR0cDovL2V4YW1wbGUuY29tL2lzX3Jvb3QiOiB0cnVlLCAiaXNzIjogImpvZSIsICJwcml2YXRlUGFyYW1zIjogeyJwcml2YXRlX29uZSI6ICJvbmUiLCAicHJpdmF0ZV90d28iOiAidHdvIn19.M10TcaFADr_JYAx7qJ71wktdyuN4IAnhWvVbgrZ5j_4")}`, Result: `{{"x":"335d1371a1400ebfc9600c7ba89ef5c24b5dcae3782009e15af55b82b6798ffe"}}`},
			},
		},
		{
			Description: "Runtime error/object insert conflict",
			Policy:      `a = { "a": y | y := [1, 2][_] }`,
			Query:       "data.p.a.a = x",
			Evals:       []Eval{{}},
			WantErr:     "internal_error: module.rego:2:5: object insert conflict",
		},
		{
			Description: "Runtime error/var assignment conflict",
			Policy: `a = "b" { input > 1 }
a = "c" { input > 2 }`,
			Query: "data.p.a = x",
			Evals: []Eval{
				{Input: "3"},
			},
			WantErr: "internal_error: module.rego:3:1: var assignment conflict",
		},
		{
			Description: "Runtime error/else conflict-1",
			Query:       `data.p.q`,
			Policy: `
				q {
					false
				}
				else = true {
					true
				}
				q = false`,
			Evals:   []Eval{{}},
			WantErr: "internal_error: module.rego:9:5: var assignment conflict",
		},
		{
			Description: "Runtime error/else conflict-2",
			Query:       `data.p.q`,
			Policy: `
				q {
					false
				}
				else = false {
					true
				}
				q {
					false
				}
				else = true {
					true
				}`,
			Evals:   []Eval{{}},
			WantErr: "internal_error: module.rego:12:5: var assignment conflict",
		},
		// NOTE(sr): The next two test cases were used to replicate issue
		// https://github.com/open-policy-agent/opa/issues/2962 -- their raison d'être
		// is thus questionable, but it might be good to keep them around a bit.
		{
			Description: "Only input changing, regex.match",
			Policy: `
			default hello = false
			hello {
				regex.match("^world$", input.message)
			}`,
			Query: "data.p.hello = x",
			Evals: []Eval{
				{Input: `{"message": "xxxxxxx"}`, Result: `{{"x": false}}`},
				{Input: `{"message": "world"}`, Result: `{{"x": true}}`},
			},
		},
		{
			Description: "Only input changing, glob.match",
			Policy: `
			default hello = false
			hello {
				glob.match("world", [":"], input.message)
			}`,
			Query: "data.p.hello = x",
			Evals: []Eval{
				{Input: `{"message": "xxxxxxx"}`, Result: `{{"x": false}}`},
				{Input: `{"message": "world"}`, Result: `{{"x": true}}`},
			},
		},
		{
			Description: "regex.match with pattern from input",
			Query:       `x = regex.match(input.re, "foo")`,
			Evals: []Eval{
				{Input: `{"re": "^foo$"}`, Result: `{{"x": true}}`},
			},
		},
		{
			Description: "regex.find_all_string_submatch_n with pattern from input",
			Query:       `x = regex.find_all_string_submatch_n(input.re, "-axxxbyc-", -1)`,
			Evals: []Eval{
				{Input: `{"re": "a(x*)b(y|z)c"}`, Result: `{{"x":[["axxxbyc","xxx","y"]]}}`},
			},
		},
		{
			Description: "simplified",
			Query:       `x := "q"; y := data.p[x]`,
			Policy: `p = 1
			q = 2`,
			Evals: []Eval{
				{Result: `{{"y": 2, "x": "q"}}`},
			},
		},
		{
			Description: "builtin sdk test",
			Query:       `x:=indexof_n("Hello World","l")`,
			Evals: []Eval{
				{Result: `{{"x": [2,3,9]}}`},
			},
		}, {
			Description: "array test",
			Query:       `x:={"c":["a","b",{"e":"d"}]}`,
			Evals: []Eval{
				{Result: `{{"x": {"c":["a","b",{"e":"d"}]}}}`},
			},
		},
		{
			Description: "mpd init problem (#3110)",
			Query:       `data.p.main = x`,
			Policy:      `main { numbers.range(1, 2)[_] == 2 }`,
			Evals: []Eval{
				{Result: `{{"x": true}}`},
				{Result: `{{"x": true}}`},
			},
		},
		{
			Description: "Virtual extent, undefined data",
			Policy: `package a.b
			c = 3`,
			Query: `data == {"a": {"b": {"c": 3 }}}`,
			Evals: []Eval{
				{Result: `{{}}`},
			},
		},
		{
			Description: "input exceeds available memory, host fails to grow it",
			Policy: `package a.b
			p = true`,
			Query:  `data.a.b.p`,
			Memory: []uint32{2, 3},
			Evals: []Eval{
				{Input: largeInput},
			},
			WantErr: "input: failed to grow memory by `2` (max pages 3)",
		},
		{
			Description: "input exceeds available memory, parsing it hits maximum",
			Policy: `package a.b
			p = true`,
			Query:  `data.a.b.p`,
			Memory: []uint32{2, 4},
			Evals: []Eval{
				{Input: largeInput},
			},
			WantErr: "internal_error: opa_malloc: failed",
		},
		{
			Description: "input exceeds available memory, grows successfully",
			Policy: `package a.b
		p = true`,
			Query:  `data.a.b.p = x`,
			Memory: []uint32{2, 8},
			Evals: []Eval{
				{Input: largeInput, Result: `{{"x":true}}`},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Description, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			policy := compileRegoToWasm(test.Policy, test.Query, dump)
			data := []byte(test.Data)
			if len(data) == 0 {
				data = nil
			}
			o := opa.New().
				WithPolicyBytes(policy).
				WithDataBytes(data).
				WithPoolSize(1) // Minimal pool size to test pooling.
			if len(test.Memory) == 2 {
				o.WithMemoryLimits(test.Memory[0]*wasm_util.PageSize, test.Memory[1]*wasm_util.PageSize)
			}

			instance, err := o.Init()
			if err != nil {
				t.Fatal(err)
			}

			// Execute each requested policy evaluation, with their inputs and updating data if requested.

			for _, eval := range test.Evals {
				switch {
				case eval.NewPolicy != "" && eval.NewData != "":
					policy := compileRegoToWasm(eval.NewPolicy, test.Query, dump)
					data := parseJSON(eval.NewData)
					if err := instance.SetPolicyData(ctx, policy, data); err != nil {
						t.Errorf(err.Error())
					}

				case eval.NewPolicy != "":
					policy := compileRegoToWasm(eval.NewPolicy, test.Query, dump)
					if err := instance.SetPolicy(ctx, policy); err != nil {
						t.Errorf(err.Error())
					}

				case eval.NewData != "":
					data := parseJSON(eval.NewData)
					if err := instance.SetData(ctx, *data); err != nil {
						t.Errorf(err.Error())
					}
				}

				r, err := instance.Eval(ctx, opa.EvalOpts{Input: parseJSON(eval.Input)})
				if err != nil {
					if test.WantErr == "" { // no error desired
						t.Fatal(err.Error())
					}
					if expected, actual := test.WantErr, err.Error(); expected != actual {
						t.Fatalf("expected error %q, got %q", expected, actual)
					}
					return
				}
				if test.WantErr != "" {
					t.Fatalf("expected error %q, got nil", test.WantErr)
				}

				expected := ast.MustParseTerm(eval.Result)
				if !ast.MustParseTerm(string(r.Result)).Equal(expected) {
					t.Errorf("\nExpected: %v\nGot: %v\n", expected, string(r.Result))
				}
			}

			instance.Close()
		})
	}
}

func TestNamedEntrypoint(t *testing.T) {
	module := `package test
	a = 7
	b = a
	`

	ctx := context.Background()

	compiler := compile.New().
		WithTarget(compile.TargetWasm).
		WithEntrypoints("test/a", "test/b").
		WithBundle(&bundle.Bundle{
			Modules: []bundle.ModuleFile{
				{
					Path:   "policy.rego",
					URL:    "policy.rego",
					Raw:    []byte(module),
					Parsed: ast.MustParseModule(module),
				},
			},
		})

	err := compiler.Build(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	instance, err := opa.New().
		WithPolicyBytes(compiler.Bundle().WasmModules[0].Raw).
		WithPoolSize(1).
		Init()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	eps, err := instance.Entrypoints(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if len(eps) != 2 {
		t.Fatalf("Expected 2 entrypoints, got: %+v", eps)
	}

	a, err := instance.Eval(ctx, opa.EvalOpts{Entrypoint: eps["test/a"]})
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	exp := ast.MustParseTerm(`{{"result":7}}`)
	actual := ast.MustParseTerm(string(a.Result))
	if !actual.Equal(exp) {
		t.Fatalf("Expected result for 'test/a' to be %s, got: %s", exp, actual)
	}

	b, err := instance.Eval(ctx, opa.EvalOpts{Entrypoint: eps["test/b"]})
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	actual = ast.MustParseTerm(string(b.Result))
	if !actual.Equal(exp) {
		t.Fatalf("Expected result for 'test/b' to be %s, got: %s", exp, actual)
	}
}

// compileRegoToWasm is shared with the benchmarking functions in opa_bench_test.go;
// those function use helpers shared with topdown_bench_test.go, and they all use
// `package test` -- whereas the callers in this file don't provide the package at
// all and assume it'll be `p`.
func compileRegoToWasm(module string, query string, dump bool) []byte {
	if !strings.HasPrefix(module, "package") {
		module = fmt.Sprintf("package p\n%s", module)
	}
	opts := []func(*rego.Rego){
		rego.Query(query),
		rego.Module("module.rego", module),
	}
	if dump {
		opts = append(opts, rego.Dump(os.Stderr))
	}
	cr, err := rego.New(opts...).Compile(context.Background(), rego.CompilePartial(false))
	if err != nil {
		panic(err)
	}

	return cr.Bytes
}

func compileRego(module string, query string) rego.PreparedEvalQuery {
	rego := rego.New(
		rego.Query(query),
		rego.Module("module.rego", module),
	)
	pq, err := rego.PrepareForEval(context.Background())
	if err != nil {
		panic(err)
	}

	return pq
}

func parseJSON(s string) *interface{} {
	if s == "" {
		return nil
	}

	v := util.MustUnmarshalJSON([]byte(s))
	return &v
}
