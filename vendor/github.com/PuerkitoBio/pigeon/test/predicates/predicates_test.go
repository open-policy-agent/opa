package predicates

import (
	"reflect"
	"testing"
)

func TestPredicatesArgs(t *testing.T) {
	methods := map[string][]string{
		"onA5":  {"a"},
		"onA9":  {"b"},
		"onA13": {"d"},
		"onB9":  {"innermost"},
		"onB10": {"inner"},
		"onB11": {"out"},
		"onC1":  {"rest"},
	}

	typ := reflect.TypeOf(&current{})
	for nm, args := range methods {
		meth, ok := typ.MethodByName(nm)
		if !ok {
			t.Errorf("want *current to have method %s", nm)
			continue
		}
		if n := meth.Func.Type().NumIn(); n != len(args)+1 {
			t.Errorf("%q: want %d arguments, got %d", nm, len(args)+1, n)
			continue
		}
	}
}
