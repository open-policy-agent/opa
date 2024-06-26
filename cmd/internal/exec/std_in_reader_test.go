package exec

import (
	"io"
	"strings"
	"testing"
)

func TestStdInReader_ReadInput(t *testing.T) {
	tcs := []struct {
		Name        string
		Reader      io.Reader
		ExpectedRes string
	}{
		{
			Name: "should read multi-line json",
			Reader: strings.NewReader(`{
"this": "that",
"those": "them"
}`),
			ExpectedRes: `{
"this": "that",
"those": "them"
}`,
		},
		{
			Name: "should read multi-line yaml",
			Reader: strings.NewReader(`this: that
those:
- them
- there`),
			ExpectedRes: `this: that
those:
- them
- there`,
		},
		{
			Name:        "should read single-line text",
			Reader:      strings.NewReader("test"),
			ExpectedRes: "test",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			sr := stdInReader{Reader: tc.Reader}
			res := sr.ReadInput()
			if res != tc.ExpectedRes {
				t.Errorf("expected read result to be %q, got %q", tc.ExpectedRes, res)
			}
		})
	}
}
