// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ir

import (
	"encoding/json"
	"fmt"
	"reflect"
)

func (a *Block) MarshalJSON() ([]byte, error) {
	var result typedBlock
	result.Stmts = make([]typedStmt, len(a.Stmts))
	for i := range a.Stmts {
		tpe := reflect.Indirect(reflect.ValueOf(a.Stmts[i])).Type().Name()
		result.Stmts[i] = typedStmt{
			Type: tpe,
			Stmt: a.Stmts[i],
		}
	}
	return json.Marshal(result)
}

func (a *Block) UnmarshalJSON(bs []byte) error {
	var typed rawTypedBlock
	if err := json.Unmarshal(bs, &typed); err != nil {
		return err
	}
	a.Stmts = make([]Stmt, len(typed.Stmts))
	for i := range typed.Stmts {
		var err error
		a.Stmts[i], err = typed.Stmts[i].Unmarshal()
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *Operand) MarshalJSON() ([]byte, error) {
	var result typedOperand
	result.Value = a.Value
	result.Type = a.Value.typeHint()
	return json.Marshal(result)
}

func (a *Operand) UnmarshalJSON(bs []byte) error {
	var typed rawTypedOperand
	if err := json.Unmarshal(bs, &typed); err != nil {
		return err
	}
	f, ok := valFactories[typed.Type]
	if !ok {
		return fmt.Errorf("unrecognized value type %q", typed.Type)
	}
	x := f()
	if err := json.Unmarshal(typed.Value, &x); err != nil {
		return err
	}
	a.Value = x
	return nil
}

type typedBlock struct {
	Stmts []typedStmt `json:"stmts"`
}

type typedStmt struct {
	Type string `json:"type"`
	Stmt Stmt   `json:"stmt"`
}

type rawTypedBlock struct {
	Stmts []rawTypedStmt `json:"stmts"`
}

type rawTypedStmt struct {
	Type string          `json:"type"`
	Stmt json.RawMessage `json:"stmt"`
}

func (raw rawTypedStmt) Unmarshal() (Stmt, error) {
	f, ok := stmtFactories[raw.Type]
	if !ok {
		return nil, fmt.Errorf("unrecognized statement type %q", raw.Type)
	}
	x := f()
	if err := json.Unmarshal(raw.Stmt, &x); err != nil {
		return nil, err
	}
	return x, nil
}

type rawTypedOperand struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

type typedOperand struct {
	Type  string `json:"type"`
	Value Val    `json:"value"`
}

// MarshalJSON for MakeNumberRefStmt emits both "index" (the canonical key,
// matching the casing of every other field in the IR) and "Index" (the
// historical key, kept for backwards compatibility with consumers that
// hard-code the original spelling). The "Index" key is deprecated and will
// be removed in a future major release; new consumers should read "index".
func (m *MakeNumberRefStmt) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		File        int   `json:"file"`
		Col         int   `json:"col"`
		Row         int   `json:"row"`
		Index       int   `json:"index"`
		IndexLegacy int   `json:"Index"` // deprecated; remove in next major
		Target      Local `json:"target"`
	}{
		File:        m.File,
		Col:         m.Col,
		Row:         m.Row,
		Index:       m.Index,
		IndexLegacy: m.Index,
		Target:      m.Target,
	})
}

// UnmarshalJSON for MakeNumberRefStmt accepts either the canonical "index"
// key or the deprecated "Index" key. When both are present, "index" wins.
func (m *MakeNumberRefStmt) UnmarshalJSON(bs []byte) error {
	var raw struct {
		File        int   `json:"file"`
		Col         int   `json:"col"`
		Row         int   `json:"row"`
		Index       *int  `json:"index"`
		IndexLegacy *int  `json:"Index"`
		Target      Local `json:"target"`
	}
	if err := json.Unmarshal(bs, &raw); err != nil {
		return err
	}
	m.File, m.Col, m.Row, m.Target = raw.File, raw.Col, raw.Row, raw.Target
	switch {
	case raw.Index != nil:
		m.Index = *raw.Index
	case raw.IndexLegacy != nil:
		m.Index = *raw.IndexLegacy
	}
	return nil
}

var stmtFactories = map[string]func() Stmt{
	"ReturnLocalStmt":      func() Stmt { return &ReturnLocalStmt{} },
	"CallStmt":             func() Stmt { return &CallStmt{} },
	"CallDynamicStmt":      func() Stmt { return &CallDynamicStmt{} },
	"BlockStmt":            func() Stmt { return &BlockStmt{} },
	"BreakStmt":            func() Stmt { return &BreakStmt{} },
	"DotStmt":              func() Stmt { return &DotStmt{} },
	"LenStmt":              func() Stmt { return &LenStmt{} },
	"ScanStmt":             func() Stmt { return &ScanStmt{} },
	"NotStmt":              func() Stmt { return &NotStmt{} },
	"AssignIntStmt":        func() Stmt { return &AssignIntStmt{} },
	"AssignVarStmt":        func() Stmt { return &AssignVarStmt{} },
	"AssignVarOnceStmt":    func() Stmt { return &AssignVarOnceStmt{} },
	"ResetLocalStmt":       func() Stmt { return &ResetLocalStmt{} },
	"MakeNullStmt":         func() Stmt { return &MakeNullStmt{} },
	"MakeNumberIntStmt":    func() Stmt { return &MakeNumberIntStmt{} },
	"MakeNumberRefStmt":    func() Stmt { return &MakeNumberRefStmt{} },
	"MakeArrayStmt":        func() Stmt { return &MakeArrayStmt{} },
	"MakeObjectStmt":       func() Stmt { return &MakeObjectStmt{} },
	"MakeSetStmt":          func() Stmt { return &MakeSetStmt{} },
	"EqualStmt":            func() Stmt { return &EqualStmt{} },
	"NotEqualStmt":         func() Stmt { return &NotEqualStmt{} },
	"IsArrayStmt":          func() Stmt { return &IsArrayStmt{} },
	"IsObjectStmt":         func() Stmt { return &IsObjectStmt{} },
	"IsDefinedStmt":        func() Stmt { return &IsDefinedStmt{} },
	"IsSetStmt":            func() Stmt { return &IsSetStmt{} },
	"IsUndefinedStmt":      func() Stmt { return &IsUndefinedStmt{} },
	"ArrayAppendStmt":      func() Stmt { return &ArrayAppendStmt{} },
	"ObjectInsertStmt":     func() Stmt { return &ObjectInsertStmt{} },
	"ObjectInsertOnceStmt": func() Stmt { return &ObjectInsertOnceStmt{} },
	"ObjectMergeStmt":      func() Stmt { return &ObjectMergeStmt{} },
	"SetAddStmt":           func() Stmt { return &SetAddStmt{} },
	"WithStmt":             func() Stmt { return &WithStmt{} },
	"NopStmt":              func() Stmt { return &NopStmt{} },
	"ResultSetAddStmt":     func() Stmt { return &ResultSetAddStmt{} },
}

var valFactories = map[string]func() Val{
	"bool":         func() Val { var x Bool; return &x },
	"string_index": func() Val { var x StringIndex; return &x },
	"local":        func() Val { var x Local; return &x },
}

// StmtKinds returns a fresh zero-value instance of every registered Stmt
// kind, keyed by the discriminator string used in the JSON form. Useful for
// tools (schema generators, linters, transformers) that need to walk the
// IR's polymorphic Stmt universe without depending on package internals.
func StmtKinds() map[string]Stmt {
	out := make(map[string]Stmt, len(stmtFactories))
	for k, f := range stmtFactories {
		out[k] = f()
	}
	return out
}

// ValKinds returns a fresh zero-value instance of every registered Val kind,
// keyed by the discriminator string. See StmtKinds for usage notes.
func ValKinds() map[string]Val {
	out := make(map[string]Val, len(valFactories))
	for k, f := range valFactories {
		out[k] = f()
	}
	return out
}
