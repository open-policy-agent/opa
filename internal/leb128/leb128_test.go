// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package leb128

import (
	"bytes"
	"testing"
)

// Test cases copied from http://dwarfstd.org/doc/Dwarf3.pdf.

func TestReadVarUint64(t *testing.T) {

	tests := []struct {
		bs  []byte
		exp uint64
	}{
		{
			bs:  []byte("\x02"),
			exp: 2,
		},
		{
			bs:  []byte("\x7F"),
			exp: 127,
		},
		{
			bs:  []byte("\x80\x01"),
			exp: 128,
		},
		{
			bs:  []byte("\x81\x01"),
			exp: 129,
		},
		{
			bs:  []byte("\x82\x01"),
			exp: 130,
		},
		{
			bs:  []byte("\xB9\x64"),
			exp: 12857,
		},
	}

	for i, tc := range tests {
		r := bytes.NewReader(tc.bs)
		result, err := ReadVarUint64(r)
		if err != nil {
			t.Fatalf("Case %d, err: %v", i, err)
		} else if result != tc.exp {
			t.Fatalf("Case %d, expected %v, but got %v", i, tc.exp, result)
		}
	}

}

func TestReadVarInt64(t *testing.T) {

	tests := []struct {
		bs  []byte
		exp int64
	}{
		{
			bs:  []byte("\x02"),
			exp: 2,
		},
		{
			bs:  []byte("\x7E"),
			exp: -2,
		},
		{
			bs:  []byte("\xFF\x00"),
			exp: 127,
		},
		{
			bs:  []byte("\x81\x7F"),
			exp: -127,
		},
		{
			bs:  []byte("\x80\x01"),
			exp: 128,
		},
		{
			bs:  []byte("\x80\x7F"),
			exp: -128,
		},
		{
			bs:  []byte("\x81\x01"),
			exp: 129,
		},
		{
			bs:  []byte("\xFF\x7E"),
			exp: -129,
		},
	}

	for i, tc := range tests {
		r := bytes.NewReader(tc.bs)
		result, err := ReadVarInt64(r)
		if err != nil {
			t.Fatalf("Case %d, err: %v", i, err)
		} else if result != tc.exp {
			t.Fatalf("Case %d, expected %v, but got %v", i, tc.exp, result)
		}
	}
}

func TestWriteVarUint64(t *testing.T) {

	tests := []struct {
		bs    []byte
		input uint64
	}{
		{
			bs:    []byte("\x02"),
			input: 2,
		},
		{
			bs:    []byte("\x7F"),
			input: 127,
		},
		{
			bs:    []byte("\x80\x01"),
			input: 128,
		},
		{
			bs:    []byte("\x81\x01"),
			input: 129,
		},
		{
			bs:    []byte("\x82\x01"),
			input: 130,
		},
		{
			bs:    []byte("\xB9\x64"),
			input: 12857,
		},
	}

	for i, tc := range tests {
		var buf bytes.Buffer

		if err := WriteVarUint64(&buf, tc.input); err != nil {
			t.Fatalf("Case %d, err: %v", i, err)
		}

		if !bytes.Equal(buf.Bytes(), tc.bs) {
			t.Fatalf("Case %d, expected %v, but got %v", i, tc.bs, buf.Bytes())
		}
	}

}

func TestWriteVarInt64(t *testing.T) {

	tests := []struct {
		bs    []byte
		input int64
	}{
		{
			bs:    []byte("\x02"),
			input: 2,
		},
		{
			bs:    []byte("\x7E"),
			input: -2,
		},
		{
			bs:    []byte("\xFF\x00"),
			input: 127,
		},
		{
			bs:    []byte("\x81\x7F"),
			input: -127,
		},
		{
			bs:    []byte("\x80\x01"),
			input: 128,
		},
		{
			bs:    []byte("\x80\x7F"),
			input: -128,
		},
		{
			bs:    []byte("\x81\x01"),
			input: 129,
		},
		{
			bs:    []byte("\xFF\x7E"),
			input: -129,
		},
	}

	for i, tc := range tests {

		var buf bytes.Buffer

		if err := WriteVarInt64(&buf, tc.input); err != nil {
			t.Fatalf("Case %d, err: %v", i, err)
		}

		if !bytes.Equal(buf.Bytes(), tc.bs) {
			t.Fatalf("Case %d, expected %v, but got %v", i, tc.bs, buf.Bytes())
		}
	}
}
