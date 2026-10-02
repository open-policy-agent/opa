package ast

import "fmt"

// RegoVersion defines the Rego syntax requirements for a module.
type RegoVersion uint8

const (
	// RegoUndefined represents a Rego version unknown to OPA, like for a policy that has
	// yet to be parsed and a no version information has been provided by other means.
	RegoUndefined RegoVersion = iota
	// RegoV0 is the original Rego syntax, which was used by default in OPA < 1.0.
	RegoV0
	// RegoV0CompatV1 requires modules to comply with both the RegoV0 and RegoV1
	// syntax (requiring Rego v1 imports in a module to use v1 keywords).
	// For more information, see https://www.openpolicyagent.org/docs/v0-compatibility
	RegoV0CompatV1
	// RegoV1 is the Rego syntax enforced by OPA 1.0, including the following changes:
	// - Keywords `in`, `every`, `ìf` and `contains` now part of the default set, and don't require explicit import
	// - Using 'if' and 'contains' now required in rule heads
	// - Most compiler checks previously enabled in "strict mode" now enabled by default
	// For more information, see https://www.openpolicyagent.org/docs/v0-upgrade
	RegoV1
	// RegoV2 adds the `and` and `or` keywords and `not` bodies to RegoV1.
	// In earlier versions, they require the `future.keywords.and`, `future.keywords.or`,
	// and `future.keywords.not` imports.
	RegoV2
)

// DefaultRegoVersion is the default Rego version for this OPA version.
const DefaultRegoVersion = RegoV1

func (v RegoVersion) Int() int {
	switch v {
	case RegoV1:
		return 1
	case RegoV2:
		return 2
	}
	return 0
}

func (v RegoVersion) String() string {
	switch v {
	case RegoV0:
		return "v0"
	case RegoV1:
		return "v1"
	case RegoV0CompatV1:
		return "v0v1"
	case RegoV2:
		return "v2"
	default:
		return "unknown"
	}
}

func RegoVersionFromInt(i int) RegoVersion {
	switch i {
	case 1:
		return RegoV1
	case 2:
		return RegoV2
	}
	return RegoV0
}

// level returns the language level of a baseline version.
func (v RegoVersion) level() (int, bool) {
	switch v {
	case RegoV0:
		return 0, true
	case RegoV1:
		return 1, true
	case RegoV2:
		return 2, true
	default:
		return 0, false
	}
}

// RegoVersionSpan is the range of Rego versions a module is compatible with:
// parsed at Min, using language features up to Max.
// The zero value is undefined.
type RegoVersionSpan struct {
	min RegoVersion
	max RegoVersion
}

// NewRegoVersionSpan returns the span [lo, hi]. An undefined hi yields [lo, lo].
// Both bounds must be RegoV0, RegoV1 or RegoV2, unless both are RegoUndefined;
// use RegoVersionSpanOf for RegoV0CompatV1.
func NewRegoVersionSpan(lo, hi RegoVersion) (RegoVersionSpan, error) {
	if lo == RegoUndefined {
		if hi == RegoUndefined {
			return RegoVersionSpan{}, nil
		}
		return RegoVersionSpan{}, fmt.Errorf("invalid rego-version span [%s, %s]: undefined min", lo, hi)
	}
	if hi == RegoUndefined {
		hi = lo
	}

	l, okl := lo.level()
	h, okh := hi.level()
	switch {
	case !okl || !okh:
		return RegoVersionSpan{}, fmt.Errorf("invalid rego-version span [%s, %s]: bounds must be v0, v1 or v2", lo, hi)
	case l > h:
		return RegoVersionSpan{}, fmt.Errorf("invalid rego-version span [%s, %s]: min > max", lo, hi)
	}

	return RegoVersionSpan{lo, hi}, nil
}

// RegoVersionSpanOf returns the span of a single version; RegoV0CompatV1 is [v0, v1].
func RegoVersionSpanOf(v RegoVersion) RegoVersionSpan {
	if v == RegoV0CompatV1 {
		return RegoVersionSpan{RegoV0, RegoV1}
	}

	return RegoVersionSpan{v, v}
}

func (s RegoVersionSpan) Min() RegoVersion {
	return s.min
}

func (s RegoVersionSpan) Max() RegoVersion {
	return s.max
}

func (s RegoVersionSpan) IsUndefined() bool {
	return s.min == RegoUndefined
}

// RegoVersion returns the legacy single value: RegoV0CompatV1 for [v0, v1] and [v0, v2],
// RegoV1 for [v1, v2], and the bound itself when min equals max.
// FIXME: Do w need this?
func (s RegoVersionSpan) RegoVersion() RegoVersion {
	switch {
	case s.min == s.max:
		return s.min
	case s.min == RegoV0:
		return RegoV0CompatV1
	}
	return s.min
}

// Int returns the integer form of Min, as recorded in bundle manifests and the store.
func (s RegoVersionSpan) Int() int {
	return s.min.Int()
}

func (s RegoVersionSpan) String() string {
	if s.min == s.max {
		return s.min.String()
	}

	return s.min.String() + s.max.String()
}
