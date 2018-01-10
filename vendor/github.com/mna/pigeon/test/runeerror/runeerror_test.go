package runeerror

import (
	"reflect"
	"strings"
	"testing"
)

var tc = `
This is a real U+FFFD: "�"
�`

var invalid = []byte{'\n', 0xff, 0xfe, 0xfd, '\n', 0xfe}

func TestRuneError(t *testing.T) {
	v, err := Parse("", []byte(tc))
	if err != nil {
		t.Error("Parsing failed:", err)
	}
	if !reflect.DeepEqual(v, [][]byte{
		[]byte(tc), []byte(`This is a real U+FFFD: "�"`), []byte(`�`),
	}) {
		t.Error("Wrong result:", v)
	}

	if _, err := Parse("", invalid); err == nil {
		t.Error("Did not fail parsing invalid encoding")
	} else if !strings.Contains(err.Error(), "invalid encoding") {
		t.Error("Unexpected error:", err)
	}

	v, err = Parse("", invalid, AllowInvalidUTF8(true))
	if err != nil {
		t.Error("Unexpected error:", err)
	}
	if !reflect.DeepEqual(v, [][]byte{
		invalid, {0xff, 0xfe, 0xfd}, {0xfe},
	}) {
		t.Error("Wrong result:", v)
	}
}
