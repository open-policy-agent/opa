package url

import (
	"errors"
	"testing"
)

func TestClean(t *testing.T) {

	cases := []struct {
		input string
		goos  string
		exp   string
		err   error
	}{
		{
			input: "c:/foo",
			exp:   "c:/foo",
			goos:  "windows",
		},
		{
			input: "file:///c:/a/b",
			exp:   "c:/a/b",
			goos:  "windows",
		},
		{
			input: "foo",
			exp:   "foo",
		},
		{
			input: "/a/b/c",
			exp:   "/a/b/c",
		},
		{
			input: "file:///a/b/c",
			exp:   "/a/b/c",
		},
		{
			input: "file://localhost/a/b/c",
			exp:   "/a/b/c",
		},
		{
			input: "file://server/share/a.rego",
			err:   errors.New("file URL host must be empty or localhost: file://server/share/a.rego"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			goos = tc.goos
			path, err := Clean(tc.input)
			if tc.err != nil {
				if err == nil {
					t.Fatalf("Want err: %v but got: %v, err: %v", tc.err, path, err)
				}
				if err.Error() != tc.err.Error() {
					t.Fatalf("Want err: %v but got: %v, err: %v", tc.err, path, err)
				}
			} else if err != nil || path != tc.exp {
				t.Fatalf("Want %v but got: %v, err: %v", tc.exp, path, err)
			}
		})
	}
}
