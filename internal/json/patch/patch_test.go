package patch

import (
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/storage"
)

func TestParsePatchPathEscaped(t *testing.T) {
	tests := []struct {
		note         string
		path         string
		expectedPath storage.Path
		expectedOK   bool
	}{
		// success-path tests
		{
			note:         "single-level",
			path:         "/single-level",
			expectedPath: storage.Path{"single-level"},
			expectedOK:   true,
		},
		{
			note:         "multi-level",
			path:         "/a/multi-level/path",
			expectedPath: storage.Path{"a", "multi-level", "path"},
			expectedOK:   true,
		},
		{
			note:         "end",
			path:         "/-",
			expectedPath: storage.Path{"-"},
			expectedOK:   true,
		},
		{ // not strictly correct but included for backwards compatibility with existing OPA
			note:         "url-escaped forward slash",
			path:         "/github.com%2Fopen-policy-agent",
			expectedPath: storage.Path{"github.com/open-policy-agent"},
			expectedOK:   true,
		},
		{
			note:         "json-pointer-escaped forward slash",
			path:         "/github.com~1open-policy-agent",
			expectedPath: storage.Path{"github.com/open-policy-agent"},
			expectedOK:   true,
		},
		{
			note:         "json-pointer-escaped tilde",
			path:         "/~0opa",
			expectedPath: storage.Path{"~opa"},
			expectedOK:   true,
		},
		{
			note:         "json-pointer-escape correctness",
			path:         "/~01",
			expectedPath: storage.Path{"~1"},
			expectedOK:   true,
		},

		// failure-path tests
		{ // not possible with existing callers but for completeness...
			note:       "empty string",
			path:       "",
			expectedOK: false,
		},
		{ // not possible with existing callers but for completeness...
			note:       "string that doesn't start with /",
			path:       "foo",
			expectedOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			actualPath, actualOK := ParsePatchPathEscaped(tc.path)

			if tc.expectedOK != actualOK {
				t.Fatalf("Expected ok to be %v but was %v", tc.expectedOK, actualOK)
			}

			if !reflect.DeepEqual(tc.expectedPath, actualPath) {
				t.Fatalf("Expected path to be %v but was %v", tc.expectedPath, actualPath)
			}
		})
	}
}
