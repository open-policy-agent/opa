package types

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v3/internal/json"
	"github.com/lestrrat-go/jwx/v3/internal/tokens"
)

const (
	DefaultPrecision uint32 = 0 // second level
	MaxPrecision     uint32 = 9 // nanosecond level
)

var Pedantic uint32
var ParsePrecision = DefaultPrecision
var FormatPrecision = DefaultPrecision

// NumericDate represents the date format used in the 'nbf' claim
type NumericDate struct {
	time.Time
}

func (n *NumericDate) Get() time.Time {
	if n == nil {
		return (time.Time{}).UTC()
	}
	return n.Time
}

func intToTime(v any, t *time.Time) bool {
	var n int64
	switch x := v.(type) {
	case int64:
		n = x
	case int32:
		n = int64(x)
	case int16:
		n = int64(x)
	case int8:
		n = int64(x)
	case int:
		n = int64(x)
	default:
		return false
	}

	*t = time.Unix(n, 0)
	return true
}

func parseNumericString(x string) (time.Time, error) {
	var t time.Time // empty time for empty return value

	// Only check for the escape hatch if it's the pedantic
	// flag is off
	if Pedantic != 1 {
		// This is an escape hatch for non-conformant providers
		// that gives us RFC3339 instead of epoch time
		for _, r := range x {
			// 0x30 = '0', 0x39 = '9', 0x2E = tokens.Period
			if (r >= 0x30 && r <= 0x39) || r == 0x2E {
				continue
			}

			// if it got here, then it probably isn't epoch time
			tv, err := time.Parse(time.RFC3339, x)
			if err != nil {
				return t, fmt.Errorf(`value is not number of seconds since the epoch, and attempt to parse it as RFC3339 timestamp failed: %w`, err)
			}
			return tv, nil
		}
	}

	var fractional string
	whole := x
	if i := strings.IndexRune(x, tokens.Period); i > 0 {
		if ParsePrecision > 0 && len(x) > i+1 {
			fractional = x[i+1:] // everything after the tokens.Period
			if int(ParsePrecision) < len(fractional) {
				// Remove insignificant digits
				fractional = fractional[:int(ParsePrecision)]
			}
			// Replace missing fractional diits with zeros
			for len(fractional) < int(MaxPrecision) {
				fractional = fractional + "0"
			}
		}
		whole = x[:i]
	}
	n, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return t, fmt.Errorf(`failed to parse whole value %q: %w`, whole, err)
	}
	var nsecs int64
	if fractional != "" {
		v, err := strconv.ParseInt(fractional, 10, 64)
		if err != nil {
			return t, fmt.Errorf(`failed to parse fractional value %q: %w`, fractional, err)
		}
		nsecs = v
	}

	return time.Unix(n, nsecs).UTC(), nil
}

func (n *NumericDate) Accept(v any) error {
	var t time.Time
	switch x := v.(type) {
	case float32:
		tv, err := parseNumericString(fmt.Sprintf(`%.9f`, x))
		if err != nil {
			return fmt.Errorf(`failed to accept float32 %.9f: %w`, x, err)
		}
		t = tv
	case float64:
		tv, err := parseNumericString(fmt.Sprintf(`%.9f`, x))
		if err != nil {
			return fmt.Errorf(`failed to accept float32 %.9f: %w`, x, err)
		}
		t = tv
	case json.Number:
		tv, err := parseNumericString(x.String())
		if err != nil {
			return fmt.Errorf(`failed to accept json.Number %q: %w`, x.String(), err)
		}
		t = tv
	case string:
		tv, err := parseNumericString(x)
		if err != nil {
			return fmt.Errorf(`failed to accept string %q: %w`, x, err)
		}
		t = tv
	case time.Time:
		t = x
	default:
		if !intToTime(v, &t) {
			return fmt.Errorf(`invalid type %T`, v)
		}
	}
	n.Time = t.UTC()
	return nil
}

func (n NumericDate) String() string {
	if FormatPrecision == 0 {
		return strconv.FormatInt(n.Unix(), 10)
	}

	// This is cheating, but it's better (easier) than doing floating point math
	// We basically munge with strings after formatting an integer value
	// for nanoseconds since epoch
	s := strconv.FormatInt(n.UnixNano(), 10)
	for len(s) < int(MaxPrecision) {
		s = "0" + s
	}

	slwhole := len(s) - int(MaxPrecision)
	s = s[:slwhole] + "." + s[slwhole:slwhole+int(FormatPrecision)]
	if s[0] == tokens.Period {
		s = "0" + s
	}

	return s
}

// MarshalJSON translates from internal representation to JSON NumericDate
// See https://tools.ietf.org/html/rfc7519#page-6
func (n *NumericDate) MarshalJSON() ([]byte, error) {
	if n.IsZero() {
		return json.Marshal(nil)
	}

	return json.Marshal(n.String())
}

func (n *NumericDate) UnmarshalJSON(data []byte) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf(`failed to unmarshal date: %w`, err)
	}

	var n2 NumericDate
	if err := n2.Accept(v); err != nil {
		return fmt.Errorf(`invalid value for NumericDate: %w`, err)
	}
	*n = n2
	return nil
}
