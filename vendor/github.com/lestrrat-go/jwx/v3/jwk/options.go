package jwk

import (
	"time"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/option/v2"
)

type identTypedField struct{}

type typedFieldPair struct {
	Name  string
	Value any
}

// WithTypedField allows a private field to be parsed into the object type of
// your choice. It works much like the RegisterCustomField, but the effect
// is only applicable to the jwt.Parse function call which receives this option.
//
// While this can be extremely useful, this option should be used with caution:
// There are many caveats that your entire team/user-base needs to be aware of,
// and therefore in general its use is discouraged. Only use it when you know
// what you are doing, and you document its use clearly for others.
//
// First and foremost, this is a "per-object" option. Meaning that given the same
// serialized format, it is possible to generate two objects whose internal
// representations may differ. That is, if you parse one _WITH_ the option,
// and the other _WITHOUT_, their internal representation may completely differ.
// This could potentially lead to problems.
//
// Second, specifying this option will slightly slow down the decoding process
// as it needs to consult multiple definitions sources (global and local), so
// be careful if you are decoding a large number of tokens, as the effects will stack up.
func WithTypedField(name string, object any) ParseOption {
	return &parseOption{
		option.New(identTypedField{},
			typedFieldPair{Name: name, Value: object},
		),
	}
}

type registerResourceOption struct {
	option.Interface
}

func (registerResourceOption) registerOption() {}
func (registerResourceOption) resourceOption() {}

type identNewResourceOption struct{}

// WithHttprcResourceOption can be used to pass arbitrary `httprc.NewResourceOption`
// to `(httprc.Client).Add` by way of `(jwk.Cache).Register`.
func WithHttprcResourceOption(o httprc.NewResourceOption) RegisterOption {
	return &registerResourceOption{
		option.New(identNewResourceOption{}, o),
	}
}

// WithConstantInterval can be used to pass `httprc.WithConstantInterval` option to
// `(httprc.Client).Add` by way of `(jwk.Cache).Register`.
func WithConstantInterval(d time.Duration) RegisterOption {
	return WithHttprcResourceOption(httprc.WithConstantInterval(d))
}

// WithMinInterval can be used to pass `httprc.WithMinInterval` option to
// `(httprc.Client).Add` by way of `(jwk.Cache).Register`.
func WithMinInterval(d time.Duration) RegisterOption {
	return WithHttprcResourceOption(httprc.WithMinInterval(d))
}

// WithMaxInterval can be used to pass `httprc.WithMaxInterval` option to
// `(httprc.Client).Add` by way of `(jwk.Cache).Register`.
func WithMaxInterval(d time.Duration) RegisterOption {
	return WithHttprcResourceOption(httprc.WithMaxInterval(d))
}
