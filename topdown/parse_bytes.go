// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"math/big"
	"strings"
	"unicode"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

const (
	none int64 = 1
	kb         = 1000
	ki         = 1024
	mb         = kb * 1000
	mi         = ki * 1024
	gb         = mb * 1000
	gi         = mi * 1024
	tb         = gb * 1000
	ti         = gi * 1024
)

func parseNumBytesError(msg string) error {
	return fmt.Errorf("%s error: %s", ast.UnitsParseBytes.Name, msg)
}

func errUnitNotRecognized(unit string) error {
	return parseNumBytesError(fmt.Sprintf("byte unit %s not recognized", unit))
}

var (
	errNoAmount       = parseNumBytesError("no byte amount provided")
	errNumConv        = parseNumBytesError("could not parse byte amount to a number")
	errIncludesSpaces = parseNumBytesError("spaces not allowed in resource strings")
)

func builtinNumBytes(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	var m big.Float

	raw, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	s := formatString(raw)

	if strings.Contains(s, " ") {
		return errIncludesSpaces
	}

	num, unit := extractNumAndUnit(s)
	if num == "" {
		return errNoAmount
	}

	switch unit {
	case "":
		m.SetInt64(none)
	case "kb", "k":
		m.SetInt64(kb)
	case "kib", "ki":
		m.SetInt64(ki)
	case "mb", "m":
		m.SetInt64(mb)
	case "mib", "mi":
		m.SetInt64(mi)
	case "gb", "g":
		m.SetInt64(gb)
	case "gib", "gi":
		m.SetInt64(gi)
	case "tb", "t":
		m.SetInt64(tb)
	case "tib", "ti":
		m.SetInt64(ti)
	default:
		return errUnitNotRecognized(unit)
	}

	numFloat, ok := new(big.Float).SetString(num)
	if !ok {
		return errNumConv
	}

	var total big.Int
	numFloat.Mul(numFloat, &m).Int(&total)
	return iter(ast.NewTerm(builtins.IntToNumber(&total)))
}

// Makes the string lower case and removes quotation marks
func formatString(s ast.String) string {
	str := string(s)
	lower := strings.ToLower(str)
	return strings.Replace(lower, "\"", "", -1)
}

// Splits the string into a number string à la "10" or "10.2" and a unit
// string à la "gb" or "MiB" or "foo". Either can be an empty string
// (error handling is provided elsewhere).
func extractNumAndUnit(s string) (string, string) {
	isNum := func(r rune) bool {
		return unicode.IsDigit(r) || r == '.'
	}

	firstNonNumIdx := -1
	for idx, r := range s {
		if !isNum(r) {
			firstNonNumIdx = idx
			break
		}
	}

	if firstNonNumIdx == -1 { // only digits and '.'
		return s, ""
	}
	if firstNonNumIdx == 0 { // only units (starts with non-digit)
		return "", s
	}

	return s[0:firstNonNumIdx], s[firstNonNumIdx:]
}

func init() {
	RegisterBuiltinFunc(ast.UnitsParseBytes.Name, builtinNumBytes)
}
