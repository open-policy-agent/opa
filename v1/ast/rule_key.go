// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"crypto/sha256"
	"encoding/binary"
	"sync"
)

// RuleKey identifies the source of a rule, see NewRuleKey. The zero value is no
// key. Experimental.
type RuleKey [16]byte

// Parsing keys every rule, so the buffers hashed are reused.
var ruleKeyBufs = sync.Pool{New: func() any { return new([]byte) }}

// NewRuleKey returns the key of a rule of package pkg (e.g. "data.a.b") from the
// source texts of its annotations and then the rule: the first 16 bytes of their
// SHA-256. Experimental.
func NewRuleKey(pkg string, texts ...[]byte) RuleKey {
	bp := ruleKeyBufs.Get().(*[]byte)
	b := binary.AppendUvarint((*bp)[:0], uint64(len(pkg)))
	b = append(b, pkg...)
	for _, t := range texts {
		b = binary.AppendUvarint(b, uint64(len(t)))
		b = append(b, t...)
	}
	sum := sha256.Sum256(b)
	*bp = b
	ruleKeyBufs.Put(bp)
	return RuleKey(sum[:])
}

// setSourceKeys records that m was parsed with annotations, so that its rules
// can be keyed by their source, and keeps the keys of rules whose source text
// is about to be dropped.
func setSourceKeys(m *Module, dropLocations bool) {
	m.keyed = true
	if !dropLocations {
		return
	}
	pkg := m.Package.Path.String()
	m.sourceKeys = make(map[*Rule]RuleKey, len(m.Rules))
	for _, r := range m.Rules {
		if k, ok := ruleSourceKeyIn(pkg, r); ok {
			m.sourceKeys[r] = k
		}
	}
}

// SourceKey returns the key of the source text rule was parsed from, see
// NewRuleKey, if it was parsed from one with annotations processed.
// Experimental.
func (rule *Rule) SourceKey() (RuleKey, bool) {
	m := rule.Module
	if m == nil || !m.keyed || m.Package == nil {
		return RuleKey{}, false
	}
	if k, ok := m.sourceKeys[rule]; ok {
		return k, true
	}
	return ruleSourceKeyIn(m.Package.Path.String(), rule)
}

func ruleSourceKeyIn(pkg string, r *Rule) (RuleKey, bool) {
	if r.Location == nil || len(r.Location.Text) == 0 {
		return RuleKey{}, false
	}
	var buf [2][]byte
	texts := buf[:0]
	for _, a := range r.Annotations {
		if a.Location == nil {
			return RuleKey{}, false
		}
		texts = append(texts, a.Location.Text)
	}
	return NewRuleKey(pkg, append(texts, r.Location.Text)...), true
}
