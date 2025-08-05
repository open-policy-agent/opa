package types

import (
	"fmt"

	"github.com/lestrrat-go/jwx/v3/internal/json"
)

type StringList []string

func (l StringList) Get() []string {
	return []string(l)
}

func (l *StringList) Accept(v any) error {
	switch x := v.(type) {
	case string:
		*l = StringList([]string{x})
	case []string:
		*l = StringList(x)
	case []any:
		list := make(StringList, len(x))
		for i, e := range x {
			if s, ok := e.(string); ok {
				list[i] = s
				continue
			}
			return fmt.Errorf(`invalid list element type %T`, e)
		}
		*l = list
	default:
		return fmt.Errorf(`invalid type: %T`, v)
	}
	return nil
}

func (l *StringList) UnmarshalJSON(data []byte) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf(`failed to unmarshal data: %w`, err)
	}
	return l.Accept(v)
}
