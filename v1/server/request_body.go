// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/open-policy-agent/opa/v1/server/authorizer"
	"github.com/open-policy-agent/opa/v1/server/types"
	"github.com/open-policy-agent/opa/v1/util"
)

// readRequest reads a request body into a T. If the authorizer already parsed the
// body, fromMap builds the T from that value instead of decoding the body again.
func readRequest[T any](r *http.Request, fromMap func(map[string]any) (T, error)) (T, error) {
	var req T
	if parsed, ok := authorizer.GetBodyOnContext(r.Context()); ok {
		m, ok := parsed.(map[string]any)
		if !ok {
			return req, fmt.Errorf("error(s) occurred while decoding request: expected an object, got %T", parsed)
		}
		return fromMap(m)
	}

	body, err := util.ReadMaybeCompressedBody(r)
	if err != nil {
		return req, readBodyError(err)
	}
	if err := util.NewJSONDecoder(bytes.NewReader(body)).Decode(&req); err != nil {
		return req, fmt.Errorf("error(s) occurred while decoding request: %w", err)
	}
	return req, nil
}

// readBodyError wraps an error from util.ReadMaybeCompressedBody. Size limit
// errors already say what was exceeded, so they are returned as is.
func readBodyError(err error) error {
	if _, ok := errors.AsType[*util.BodyTooLargeError](err); ok {
		return err
	}
	return fmt.Errorf("could not decompress the body: %w", err)
}

func isEmptyBody(err error) bool {
	return errors.Is(err, io.EOF)
}

// fields reads typed values out of a parsed JSON object, remembering the first
// type mismatch.
type fields struct {
	m   map[string]any
	err error
}

func get[T any](f *fields, key string) T {
	var zero T
	v, ok := f.m[key]
	if !ok || v == nil || f.err != nil {
		return zero
	}
	t, ok := v.(T)
	if !ok {
		f.err = fmt.Errorf("error(s) occurred while decoding request: %s: unexpected %T", key, v)
		return zero
	}
	return t
}

// strings returns nil if the key is absent or null.
func (f *fields) strings(key string) []string {
	vs := get[[]any](f, key)
	if vs == nil {
		return nil
	}
	out := make([]string, len(vs))
	for i, v := range vs {
		s, ok := v.(string)
		if !ok && f.err == nil {
			f.err = fmt.Errorf("error(s) occurred while decoding request: %s: unexpected %T", key, v)
		}
		out[i] = s
	}
	return out
}

func (f *fields) input() *any {
	if v := f.m["input"]; v != nil {
		return &v
	}
	return nil
}

func (f *fields) unknowns() *[]string {
	if us := f.strings("unknowns"); us != nil {
		return &us
	}
	return nil
}

func queryRequestFromMap(m map[string]any) (types.QueryRequestV1, error) {
	f := fields{m: m}
	return types.QueryRequestV1{Query: get[string](&f, "query"), Input: f.input()}, f.err
}

func compileRequestFromMap(m map[string]any) (types.CompileRequestV1, error) {
	f := fields{m: m}
	o := fields{m: get[map[string]any](&f, "options")}

	req := types.CompileRequestV1{
		Input:    f.input(),
		Query:    get[string](&f, "query"),
		Unknowns: f.unknowns(),
	}
	req.Options.DisableInlining = o.strings("disableInlining")
	req.Options.NondeterministicBuiltins = get[bool](&o, "nondeterministicBuiltins")
	return req, errors.Join(f.err, o.err)
}

func compileFiltersRequestFromMap(m map[string]any) (CompileFiltersRequestV1, error) {
	f := fields{m: m}
	o := fields{m: get[map[string]any](&f, "options")}

	req := CompileFiltersRequestV1{
		Input:    f.input(),
		Query:    get[string](&f, "query"),
		Unknowns: f.unknowns(),
		Metadata: types.Extras[CompileFiltersRequestV1](m),
	}
	req.Options.DisableInlining = o.strings("disableInlining")
	req.Options.Mappings = get[map[string]any](&o, "targetSQLTableMappings")
	req.Options.TargetDialects = o.strings("targetDialects")
	req.Options.MaskRule = get[string](&o, "maskRule")
	return req, errors.Join(f.err, o.err)
}
