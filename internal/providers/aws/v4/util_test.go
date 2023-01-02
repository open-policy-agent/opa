package v4

import (
	"testing"
)

func TestStripExcessHeaders(t *testing.T) {
	vals := []string{
		"",
		"123",
		"1 2 3",
		"1 2 3 ",
		"  1 2 3",
		"1  2 3",
		"1  23",
		"1  2  3",
		"1  2  ",
		" 1  2  ",
		"12   3",
		"12   3   1",
		"12           3     1",
		"12     3       1abc123",
	}

	expected := []string{
		"",
		"123",
		"1 2 3",
		"1 2 3",
		"1 2 3",
		"1 2 3",
		"1 23",
		"1 2 3",
		"1 2",
		"1 2",
		"12 3",
		"12 3 1",
		"12 3 1",
		"12 3 1abc123",
	}

	for i := 0; i < len(vals); i++ {
		r := StripExcessSpaces(vals[i])
		if e, a := expected[i], r; e != a {
			t.Errorf("%d, expect %v, got %v", i, e, a)
		}
	}
}

var stripExcessSpaceCases = []string{
	`AWS4-HMAC-SHA256 Credential=AKIDFAKEIDFAKEID/20160628/us-west-2/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=1234567890abcdef1234567890abcdef1234567890abcdef`,
	`123   321   123   321`,
	`   123   321   123   321   `,
	`   123    321    123          321   `,
	"123",
	"1 2 3",
	"  1 2 3",
	"1  2 3",
	"1  23",
	"1  2  3",
	"1  2  ",
	" 1  2  ",
	"12   3",
	"12   3   1",
	"12           3     1",
	"12     3       1abc123",
}

func BenchmarkStripExcessSpaces(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, v := range stripExcessSpaceCases {
			StripExcessSpaces(v)
		}
	}
}
