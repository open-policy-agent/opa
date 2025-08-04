package jwt

import (
	"fmt"

	"github.com/lestrrat-go/jwx/v3/internal/json"
	"github.com/lestrrat-go/jwx/v3/jwe"
	"github.com/lestrrat-go/jwx/v3/jws"
)

type SerializeCtx interface {
	Step() int
	Nested() bool
}

type serializeCtx struct {
	step   int
	nested bool
}

func (ctx *serializeCtx) Step() int {
	return ctx.step
}

func (ctx *serializeCtx) Nested() bool {
	return ctx.nested
}

type SerializeStep interface {
	Serialize(SerializeCtx, any) (any, error)
}

// errStep is always an error. used to indicate that a method like
// serializer.Sign or Encrypt already errored out on configuration
type errStep struct {
	err error
}

func (e errStep) Serialize(_ SerializeCtx, _ any) (any, error) {
	return nil, e.err
}

// Serializer is a generic serializer for JWTs. Whereas other convenience
// functions can only do one thing (such as generate a JWS signed JWT),
// Using this construct you can serialize the token however you want.
//
// By default, the serializer only marshals the token into a JSON payload.
// You must set up the rest of the steps that should be taken by the
// serializer.
//
// For example, to marshal the token into JSON, then apply JWS and JWE
// in that order, you would do:
//
//	serialized, err := jwt.NewSerializer().
//	   Sign(jwa.RS256, key).
//	   Encrypt(jwe.WithEncryptOption(jwe.WithKey(jwa.RSA_OAEP(), publicKey))).
//	   Serialize(token)
//
// The `jwt.Sign()` function is equivalent to
//
//	serialized, err := jwt.NewSerializer().
//	   Sign(...args...).
//	   Serialize(token)
type Serializer struct {
	steps []SerializeStep
}

// NewSerializer creates a new empty serializer.
func NewSerializer() *Serializer {
	return &Serializer{}
}

// Reset clears all of the registered steps.
func (s *Serializer) Reset() *Serializer {
	s.steps = nil
	return s
}

// Step adds a new Step to the serialization process
func (s *Serializer) Step(step SerializeStep) *Serializer {
	s.steps = append(s.steps, step)
	return s
}

type jsonSerializer struct{}

func (jsonSerializer) Serialize(_ SerializeCtx, v any) (any, error) {
	token, ok := v.(Token)
	if !ok {
		return nil, fmt.Errorf(`invalid input: expected jwt.Token`)
	}

	buf, err := json.Marshal(token)
	if err != nil {
		return nil, fmt.Errorf(`failed to serialize as JSON: %w`, err)
	}
	return buf, nil
}

type genericHeader interface {
	Get(string, any) error
	Set(string, any) error
	Has(string) bool
}

func setTypeOrCty(ctx SerializeCtx, hdrs genericHeader) error {
	// cty and typ are common between JWE/JWS, so we don't use
	// the constants in jws/jwe package here
	const typKey = `typ`
	const ctyKey = `cty`

	if ctx.Step() == 1 {
		// We are executed immediately after json marshaling
		if !hdrs.Has(typKey) {
			if err := hdrs.Set(typKey, `JWT`); err != nil {
				return fmt.Errorf(`failed to set %s key to "JWT": %w`, typKey, err)
			}
		}
	} else {
		if ctx.Nested() {
			// If this is part of a nested sequence, we should set cty = 'JWT'
			// https://datatracker.ietf.org/doc/html/rfc7519#section-5.2
			if err := hdrs.Set(ctyKey, `JWT`); err != nil {
				return fmt.Errorf(`failed to set %s key to "JWT": %w`, ctyKey, err)
			}
		}
	}
	return nil
}

type jwsSerializer struct {
	options []jws.SignOption
}

func (s *jwsSerializer) Serialize(ctx SerializeCtx, v any) (any, error) {
	payload, ok := v.([]byte)
	if !ok {
		return nil, fmt.Errorf(`expected []byte as input`)
	}

	for _, option := range s.options {
		var pc interface{ Protected(jws.Headers) jws.Headers }
		if err := option.Value(&pc); err != nil {
			continue
		}
		hdrs := pc.Protected(jws.NewHeaders())
		if err := setTypeOrCty(ctx, hdrs); err != nil {
			return nil, err // this is already wrapped
		}

		// JWTs MUST NOT use b64 = false
		// https://datatracker.ietf.org/doc/html/rfc7797#section-7
		var b64 bool
		if err := hdrs.Get("b64", &b64); err == nil {
			if !b64 { // b64 = false
				return nil, fmt.Errorf(`b64 cannot be false for JWTs`)
			}
		}
	}
	return jws.Sign(payload, s.options...)
}

func (s *Serializer) Sign(options ...SignOption) *Serializer {
	var soptions []jws.SignOption
	if l := len(options); l > 0 {
		// we need to from SignOption to Option because ... reasons
		// (todo: when go1.18 prevails, use type parameters
		rawoptions := make([]Option, l)
		for i, option := range options {
			rawoptions[i] = option
		}

		converted, err := toSignOptions(rawoptions...)
		if err != nil {
			return s.Step(errStep{fmt.Errorf(`(jwt.Serializer).Sign: failed to convert options into jws.SignOption: %w`, err)})
		}
		soptions = converted
	}
	return s.sign(soptions...)
}

func (s *Serializer) sign(options ...jws.SignOption) *Serializer {
	return s.Step(&jwsSerializer{
		options: options,
	})
}

type jweSerializer struct {
	options []jwe.EncryptOption
}

func (s *jweSerializer) Serialize(ctx SerializeCtx, v any) (any, error) {
	payload, ok := v.([]byte)
	if !ok {
		return nil, fmt.Errorf(`expected []byte as input`)
	}

	hdrs := jwe.NewHeaders()
	if err := setTypeOrCty(ctx, hdrs); err != nil {
		return nil, err // this is already wrapped
	}

	options := append([]jwe.EncryptOption{jwe.WithMergeProtectedHeaders(true), jwe.WithProtectedHeaders(hdrs)}, s.options...)
	return jwe.Encrypt(payload, options...)
}

// Encrypt specifies the JWT to be serialized as an encrypted payload.
//
// One notable difference between this method and `jwe.Encrypt()` is that
// while `jwe.Encrypt()` OVERWRITES the previous headers when `jwe.WithProtectedHeaders()`
// is provided, this method MERGES them. This is due to the fact that we
// MUST add some extra headers to construct a proper JWE message.
// Be careful when you pass multiple `jwe.EncryptOption`s.
func (s *Serializer) Encrypt(options ...EncryptOption) *Serializer {
	var eoptions []jwe.EncryptOption
	if l := len(options); l > 0 {
		// we need to from SignOption to Option because ... reasons
		// (todo: when go1.18 prevails, use type parameters
		rawoptions := make([]Option, l)
		for i, option := range options {
			rawoptions[i] = option
		}

		converted, err := toEncryptOptions(rawoptions...)
		if err != nil {
			return s.Step(errStep{fmt.Errorf(`(jwt.Serializer).Encrypt: failed to convert options into jwe.EncryptOption: %w`, err)})
		}
		eoptions = converted
	}
	return s.encrypt(eoptions...)
}

func (s *Serializer) encrypt(options ...jwe.EncryptOption) *Serializer {
	return s.Step(&jweSerializer{
		options: options,
	})
}

func (s *Serializer) Serialize(t Token) ([]byte, error) {
	steps := make([]SerializeStep, len(s.steps)+1)
	steps[0] = jsonSerializer{}
	for i, step := range s.steps {
		steps[i+1] = step
	}

	var ctx serializeCtx
	ctx.nested = len(s.steps) > 1
	var payload any = t
	for i, step := range steps {
		ctx.step = i
		v, err := step.Serialize(&ctx, payload)
		if err != nil {
			return nil, fmt.Errorf(`failed to serialize token at step #%d: %w`, i+1, err)
		}
		payload = v
	}

	res, ok := payload.([]byte)
	if !ok {
		return nil, fmt.Errorf(`invalid serialization produced`)
	}

	return res, nil
}
