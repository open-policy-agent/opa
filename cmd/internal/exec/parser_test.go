package exec

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

type errReader int

func (errReader) Read(_ []byte) (n int, err error) {
	return 0, errors.New("test error")
}

func TestUtilParser_Parse(t *testing.T) {
	testJSON := map[string]string{"this": "that"}
	b, err := json.Marshal(testJSON)
	if err != nil {
		t.Fatalf("unexpected error marshalling valid json: %q", err.Error())
	}
	tcs := []struct {
		Name        string
		Reader      io.Reader
		ShouldError bool
		Expectation func(x interface{})
	}{
		{
			Name:        "should return an error if the provided reader raises an error",
			Reader:      new(errReader),
			ShouldError: true,
		},
		{
			Name:        "should return an error for invalid JSON",
			Reader:      strings.NewReader("{[invalid json"),
			ShouldError: true,
		},
		{
			Name:        "should return a valid JSON object",
			Reader:      bytes.NewBuffer(b),
			ShouldError: false,
			Expectation: func(x interface{}) {
				if val, ok := x.(map[string]interface{})["this"]; !ok {
					t.Fatalf("expected returned value to have key %q, but none was found", "this")
				} else if val != "that" {
					t.Fatalf("expected returned value to have value %q for key %q, instead got %q", "that", "this", val)
				}
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			up := utilParser{}
			res, err := up.Parse(tc.Reader)
			if tc.ShouldError {
				if err == nil {
					t.Fatalf("expected error, found none")
				}
			} else {
				tc.Expectation(res)
			}
		})
	}
}
