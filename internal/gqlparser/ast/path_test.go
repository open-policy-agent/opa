package ast

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPath_String(t *testing.T) {
	type Spec struct {
		Value    Path
		Expected string
	}
	specs := []*Spec{
		{
			Value:    Path{PathName("a"), PathIndex(2), PathName("c")},
			Expected: "a[2].c",
		},
		{
			Value:    Path{},
			Expected: ``,
		},
		{
			Value:    Path{PathIndex(1), PathName("b")},
			Expected: `[1].b`,
		},
	}

	for _, spec := range specs {
		t.Run(spec.Value.String(), func(t *testing.T) {
			require.Equal(t, spec.Expected, spec.Value.String())
		})
	}
}

func TestPath_MarshalJSON(t *testing.T) {
	type Spec struct {
		Value    Path
		Expected string
	}
	specs := []*Spec{
		{
			Value:    Path{PathName("a"), PathIndex(2), PathName("c")},
			Expected: `["a",2,"c"]`,
		},
		{
			Value:    Path{},
			Expected: `[]`,
		},
		{
			Value:    Path{PathIndex(1), PathName("b")},
			Expected: `[1,"b"]`,
		},
	}

	for _, spec := range specs {
		t.Run(spec.Value.String(), func(t *testing.T) {
			b, err := json.Marshal(spec.Value)
			require.Nil(t, err)

			require.Equal(t, spec.Expected, string(b))
		})
	}
}

func TestPath_UnmarshalJSON(t *testing.T) {
	type Spec struct {
		Value    string
		Expected Path
	}
	specs := []*Spec{
		{
			Value:    `["a",2,"c"]`,
			Expected: Path{PathName("a"), PathIndex(2), PathName("c")},
		},
		{
			Value:    `[]`,
			Expected: Path{},
		},
		{
			Value:    `[1,"b"]`,
			Expected: Path{PathIndex(1), PathName("b")},
		},
	}

	for _, spec := range specs {
		t.Run(spec.Value, func(t *testing.T) {
			var path Path
			err := json.Unmarshal([]byte(spec.Value), &path)
			require.Nil(t, err)

			require.Equal(t, spec.Expected, path)
		})
	}
}
