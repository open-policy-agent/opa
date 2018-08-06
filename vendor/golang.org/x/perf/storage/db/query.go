// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package db

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

// operation is the enum for possible query operations.
type operation rune

// Query operations.
// Only equals, lt, and gt can be specified in a query.
// The order of these operations is used by merge.
const (
	equals operation = iota
	ltgt
	lt
	gt
)

// A part is a single query part with a key, operator, and value.
type part struct {
	key      string
	operator operation
	// value and value2 hold the values to compare against.
	value, value2 string
}

// sepToOperation maps runes to operation values.
var sepToOperation = map[byte]operation{
	':': equals,
	'<': lt,
	'>': gt,
}

// parseWord parse a single query part (as returned by SplitWords with quoting and escaping already removed) into a part struct.
func parseWord(word string) (part, error) {
	sepIndex := strings.IndexFunc(word, func(r rune) bool {
		return r == ':' || r == '>' || r == '<' || unicode.IsSpace(r) || unicode.IsUpper(r)
	})
	if sepIndex < 0 {
		return part{}, fmt.Errorf("query part %q is missing operator", word)
	}
	key, sep, value := word[:sepIndex], word[sepIndex], word[sepIndex+1:]
	if oper, ok := sepToOperation[sep]; ok {
		return part{key, oper, value, ""}, nil
	}
	return part{}, fmt.Errorf("query part %q has invalid key", word)
}

// merge merges two query parts together into a single query part.
// The keys of the two parts must be equal.
// If the result is a query part that can never match, io.EOF is returned as the error.
func (p part) merge(p2 part) (part, error) {
	if p2.operator < p.operator {
		// Sort the parts so we only need half the table below.
		p, p2 = p2, p
	}
	switch p.operator {
	case equals:
		switch p2.operator {
		case equals:
			if p.value == p2.value {
				return p, nil
			}
			return part{}, io.EOF
		case lt:
			if p.value < p2.value {
				return p, nil
			}
			return part{}, io.EOF
		case gt:
			if p.value > p2.value {
				return p, nil
			}
			return part{}, io.EOF
		case ltgt:
			if p.value < p2.value && p.value > p2.value2 {
				return p, nil
			}
			return part{}, io.EOF
		}
	case ltgt:
		switch p2.operator {
		case ltgt:
			if p2.value < p.value {
				p.value = p2.value
			}
			if p2.value2 > p.value2 {
				p.value2 = p2.value2
			}
		case lt:
			if p2.value < p.value {
				p.value = p2.value
			}
		case gt:
			if p2.value > p.value2 {
				p.value2 = p2.value
			}
		}
	case lt:
		switch p2.operator {
		case lt:
			if p2.value < p.value {
				return p2, nil
			}
			return p, nil
		case gt:
			p = part{p.key, ltgt, p.value, p2.value}
		}
	case gt:
		// p2.operator == gt
		if p2.value > p.value {
			return p2, nil
		}
		return p, nil
	}
	// p.operator == ltgt
	if p.value <= p.value2 || p.value == "" {
		return part{}, io.EOF
	}
	if p.value2 == "" {
		return part{p.key, lt, p.value, ""}, nil
	}
	return p, nil
}

// sql returns a SQL expression and a list of arguments for finding records matching p.
func (p part) sql() (sql string, args []interface{}, err error) {
	if p.key == "upload" {
		switch p.operator {
		case equals:
			return "SELECT UploadID, RecordID FROM Records WHERE UploadID = ?", []interface{}{p.value}, nil
		case lt:
			return "SELECT UploadID, RecordID FROM Records WHERE UploadID < ?", []interface{}{p.value}, nil
		case gt:
			return "SELECT UploadID, RecordID FROM Records WHERE UploadID > ?", []interface{}{p.value}, nil
		case ltgt:
			return "SELECT UploadID, RecordID FROM Records WHERE UploadID < ? AND UploadID > ?", []interface{}{p.value, p.value2}, nil
		}
	}
	switch p.operator {
	case equals:
		if p.value == "" {
			// TODO(quentin): Implement support for searching for missing labels.
			return "", nil, fmt.Errorf("missing value for key %q", p.key)
		}
		return "SELECT UploadID, RecordID FROM RecordLabels WHERE Name = ? AND Value = ?", []interface{}{p.key, p.value}, nil
	case lt:
		return "SELECT UploadID, RecordID FROM RecordLabels WHERE Name = ? AND Value < ?", []interface{}{p.key, p.value}, nil
	case gt:
		if p.value == "" {
			// Simplify queries for any value.
			return "SELECT UploadID, RecordID FROM RecordLabels WHERE Name = ?", []interface{}{p.key}, nil
		}
		return "SELECT UploadID, RecordID FROM RecordLabels WHERE Name = ? AND Value > ?", []interface{}{p.key, p.value}, nil
	case ltgt:
		return "SELECT UploadID, RecordID FROM RecordLabels WHERE Name = ? AND Value < ? AND Value > ?", []interface{}{p.key, p.value, p.value2}, nil
	default:
		panic("unknown operator " + string(p.operator))
	}
}
