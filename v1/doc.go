// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package v1 implements the v1 API for the Open Policy Agent (OPA).
// The v1 API defaults to enforcing the v1 Rego syntax ([github.com/open-policy-agent/opa/v1/ast.RegoV1]).
// Most packages outside the v1 API are deprecated. These constitute the older v0 API, which defaults to the v0 Rego syntax ([github.com/open-policy-agent/opa/v1/ast.RegoV0]).
// The v0 API is provided as a means to ease transition to OPA 1.0 for 3rd party integrations, see [TODO: LINK TO V0 MIGRATION GUIDE].
package v1
