package jwt

import (
	"context"
	"fmt"
	"strconv"
	"time"

	jwterrs "github.com/lestrrat-go/jwx/v3/jwt/internal/errors"
)

type Clock interface {
	Now() time.Time
}
type ClockFunc func() time.Time

func (f ClockFunc) Now() time.Time {
	return f()
}

func isSupportedTimeClaim(c string) error {
	switch c {
	case ExpirationKey, IssuedAtKey, NotBeforeKey:
		return nil
	}
	return fmt.Errorf(`unsupported time claim %s`, strconv.Quote(c))
}

func timeClaim(t Token, clock Clock, c string) time.Time {
	// We don't check if the claims already exist. It should have been done
	// by piggybacking on `required` check.
	switch c {
	case ExpirationKey:
		tv, _ := t.Expiration()
		return tv
	case IssuedAtKey:
		tv, _ := t.IssuedAt()
		return tv
	case NotBeforeKey:
		tv, _ := t.NotBefore()
		return tv
	case "":
		return clock.Now()
	}
	return time.Time{} // should *NEVER* reach here, but...
}

// Validate makes sure that the essential claims stand.
//
// See the various `WithXXX` functions for optional parameters
// that can control the behavior of this method.
func Validate(t Token, options ...ValidateOption) error {
	ctx := context.Background()
	trunc := getDefaultTruncation()

	var clock Clock = ClockFunc(time.Now)
	var skew time.Duration
	var baseValidators = []Validator{
		IsIssuedAtValid(),
		IsExpirationValid(),
		IsNbfValid(),
	}
	var extraValidators []Validator
	var resetValidators bool
	for _, o := range options {
		switch o.Ident() {
		case identClock{}:
			if err := o.Value(&clock); err != nil {
				return fmt.Errorf(`jwt.Validate: value for WithClock() option must be jwt.Clock: %w`, err)
			}
		case identAcceptableSkew{}:
			if err := o.Value(&skew); err != nil {
				return fmt.Errorf(`jwt.Validate: value for WithAcceptableSkew() option must be time.Duration: %w`, err)
			}
		case identTruncation{}:
			if err := o.Value(&trunc); err != nil {
				return fmt.Errorf(`jwt.Validate: value for WithTruncation() option must be time.Duration: %w`, err)
			}
		case identContext{}:
			if err := o.Value(&ctx); err != nil {
				return fmt.Errorf(`jwt.Validate: value for WithContext() option must be context.Context: %w`, err)
			}
		case identResetValidators{}:
			if err := o.Value(&resetValidators); err != nil {
				return fmt.Errorf(`jwt.Validate: value for WithResetValidators() option must be bool: %w`, err)
			}
		case identValidator{}:
			var v Validator
			if err := o.Value(&v); err != nil {
				return fmt.Errorf(`jwt.Validate: value for WithValidator() option must be jwt.Validator: %w`, err)
			}
			switch v := v.(type) {
			case *isInTimeRange:
				if v.c1 != "" {
					if err := isSupportedTimeClaim(v.c1); err != nil {
						return err
					}
					extraValidators = append(extraValidators, IsRequired(v.c1))
				}
				if v.c2 != "" {
					if err := isSupportedTimeClaim(v.c2); err != nil {
						return err
					}
					extraValidators = append(extraValidators, IsRequired(v.c2))
				}
			}
			extraValidators = append(extraValidators, v)
		}
	}

	ctx = SetValidationCtxSkew(ctx, skew)
	ctx = SetValidationCtxClock(ctx, clock)
	ctx = SetValidationCtxTruncation(ctx, trunc)

	var validators []Validator
	if !resetValidators {
		validators = append(baseValidators, extraValidators...)
	} else {
		if len(extraValidators) == 0 {
			return jwterrs.ValidateErrorf(`no validators specified: jwt.WithResetValidators(true) and no jwt.WithValidator() specified`)
		}
		validators = extraValidators
	}

	for _, v := range validators {
		if err := v.Validate(ctx, t); err != nil {
			return jwterrs.ValidateErrorf(`validation failed: %w`, err)
		}
	}

	return nil
}

type isInTimeRange struct {
	c1   string
	c2   string
	dur  time.Duration
	less bool // if true, d =< c1 - c2. otherwise d >= c1 - c2
}

// MaxDeltaIs implements the logic behind `WithMaxDelta()` option
func MaxDeltaIs(c1, c2 string, dur time.Duration) Validator {
	return &isInTimeRange{
		c1:   c1,
		c2:   c2,
		dur:  dur,
		less: true,
	}
}

// MinDeltaIs implements the logic behind `WithMinDelta()` option
func MinDeltaIs(c1, c2 string, dur time.Duration) Validator {
	return &isInTimeRange{
		c1:   c1,
		c2:   c2,
		dur:  dur,
		less: false,
	}
}

func (iitr *isInTimeRange) Validate(ctx context.Context, t Token) error {
	clock := ValidationCtxClock(ctx) // MUST be populated
	skew := ValidationCtxSkew(ctx)   // MUST be populated
	// We don't check if the claims already exist, because we already did that
	// by piggybacking on `required` check.
	t1 := timeClaim(t, clock, iitr.c1)
	t2 := timeClaim(t, clock, iitr.c2)
	if iitr.less { // t1 - t2 <= iitr.dur
		// t1 - t2 < iitr.dur + skew
		if t1.Sub(t2) > iitr.dur+skew {
			return fmt.Errorf(`iitr between %s and %s exceeds %s (skew %s)`, iitr.c1, iitr.c2, iitr.dur, skew)
		}
	} else {
		if t1.Sub(t2) < iitr.dur-skew {
			return fmt.Errorf(`iitr between %s and %s is less than %s (skew %s)`, iitr.c1, iitr.c2, iitr.dur, skew)
		}
	}
	return nil
}

// Validator describes interface to validate a Token.
type Validator interface {
	// Validate should return an error if a required conditions is not met.
	Validate(context.Context, Token) error
}

// ValidatorFunc is a type of Validator that does not have any
// state, that is implemented as a function
type ValidatorFunc func(context.Context, Token) error

func (vf ValidatorFunc) Validate(ctx context.Context, tok Token) error {
	return vf(ctx, tok)
}

type identValidationCtxClock struct{}
type identValidationCtxSkew struct{}
type identValidationCtxTruncation struct{}

func SetValidationCtxClock(ctx context.Context, cl Clock) context.Context {
	return context.WithValue(ctx, identValidationCtxClock{}, cl)
}

func SetValidationCtxTruncation(ctx context.Context, dur time.Duration) context.Context {
	return context.WithValue(ctx, identValidationCtxTruncation{}, dur)
}

func SetValidationCtxSkew(ctx context.Context, dur time.Duration) context.Context {
	return context.WithValue(ctx, identValidationCtxSkew{}, dur)
}

// ValidationCtxClock returns the Clock object associated with
// the current validation context. This value will always be available
// during validation of tokens.
func ValidationCtxClock(ctx context.Context) Clock {
	//nolint:forcetypeassert
	return ctx.Value(identValidationCtxClock{}).(Clock)
}

func ValidationCtxSkew(ctx context.Context) time.Duration {
	//nolint:forcetypeassert
	return ctx.Value(identValidationCtxSkew{}).(time.Duration)
}

func ValidationCtxTruncation(ctx context.Context) time.Duration {
	//nolint:forcetypeassert
	return ctx.Value(identValidationCtxTruncation{}).(time.Duration)
}

// IsExpirationValid is one of the default validators that will be executed.
// It does not need to be specified by users, but it exists as an
// exported field so that you can check what it does.
//
// The supplied context.Context object must have the "clock" and "skew"
// populated with appropriate values using SetValidationCtxClock() and
// SetValidationCtxSkew()
func IsExpirationValid() Validator {
	return ValidatorFunc(isExpirationValid)
}

func isExpirationValid(ctx context.Context, t Token) error {
	tv, ok := t.Expiration()
	if !ok {
		return nil
	}

	clock := ValidationCtxClock(ctx)      // MUST be populated
	skew := ValidationCtxSkew(ctx)        // MUST be populated
	trunc := ValidationCtxTruncation(ctx) // MUST be populated

	now := clock.Now().Truncate(trunc)
	ttv := tv.Truncate(trunc)

	// expiration date must be after NOW
	if !now.Before(ttv.Add(skew)) {
		return TokenExpiredError()
	}
	return nil
}

// IsIssuedAtValid is one of the default validators that will be executed.
// It does not need to be specified by users, but it exists as an
// exported field so that you can check what it does.
//
// The supplied context.Context object must have the "clock" and "skew"
// populated with appropriate values using SetValidationCtxClock() and
// SetValidationCtxSkew()
func IsIssuedAtValid() Validator {
	return ValidatorFunc(isIssuedAtValid)
}

func isIssuedAtValid(ctx context.Context, t Token) error {
	tv, ok := t.IssuedAt()
	if !ok {
		return nil
	}

	clock := ValidationCtxClock(ctx)      // MUST be populated
	skew := ValidationCtxSkew(ctx)        // MUST be populated
	trunc := ValidationCtxTruncation(ctx) // MUST be populated

	now := clock.Now().Truncate(trunc)
	ttv := tv.Truncate(trunc)

	if now.Before(ttv.Add(-1 * skew)) {
		return InvalidIssuedAtError()
	}
	return nil
}

// IsNbfValid is one of the default validators that will be executed.
// It does not need to be specified by users, but it exists as an
// exported field so that you can check what it does.
//
// The supplied context.Context object must have the "clock" and "skew"
// populated with appropriate values using SetValidationCtxClock() and
// SetValidationCtxSkew()
func IsNbfValid() Validator {
	return ValidatorFunc(isNbfValid)
}

func isNbfValid(ctx context.Context, t Token) error {
	tv, ok := t.NotBefore()
	if !ok {
		return nil
	}

	clock := ValidationCtxClock(ctx)      // MUST be populated
	skew := ValidationCtxSkew(ctx)        // MUST be populated
	trunc := ValidationCtxTruncation(ctx) // MUST be populated

	// Truncation always happens even for trunc = 0 because
	// we also use this to strip monotonic clocks
	now := clock.Now().Truncate(trunc)
	ttv := tv.Truncate(trunc)

	// "now" cannot be before t - skew, so we check for now > t - skew
	ttv = ttv.Add(-1 * skew)
	if now.Before(ttv) {
		return TokenNotYetValidError()
	}
	return nil
}

type claimContainsString struct {
	name    string
	value   string
	makeErr func(string, ...any) error
}

// ClaimContainsString can be used to check if the claim called `name`, which is
// expected to be a list of strings, contains `value`. Currently, because of the
// implementation, this will probably only work for `aud` fields.
func ClaimContainsString(name, value string) Validator {
	return claimContainsString{
		name:    name,
		value:   value,
		makeErr: fmt.Errorf,
	}
}

func (ccs claimContainsString) Validate(_ context.Context, t Token) error {
	var list []string
	if err := t.Get(ccs.name, &list); err != nil {
		return ccs.makeErr(`claim %q does not exist or is not a []string: %w`, ccs.name, err)
	}

	for _, v := range list {
		if v == ccs.value {
			return nil
		}
	}
	return ccs.makeErr(`%q not satisfied`, ccs.name)
}

// audienceClaimContainsString can be used to check if the audience claim, which is
// expected to be a list of strings, contains `value`.
func audienceClaimContainsString(value string) Validator {
	return claimContainsString{
		name:    AudienceKey,
		value:   value,
		makeErr: jwterrs.AudienceErrorf,
	}
}

type claimValueIs struct {
	name    string
	value   any
	makeErr func(string, ...any) error
}

// ClaimValueIs creates a Validator that checks if the value of claim `name`
// matches `value`. The comparison is done using a simple `==` comparison,
// and therefore complex comparisons may fail using this code. If you
// need to do more, use a custom Validator.
func ClaimValueIs(name string, value any) Validator {
	return &claimValueIs{
		name:    name,
		value:   value,
		makeErr: fmt.Errorf,
	}
}

func (cv *claimValueIs) Validate(_ context.Context, t Token) error {
	var v any
	if err := t.Get(cv.name, &v); err != nil {
		return cv.makeErr(`claim %[1]q does not exist or is not a []string: %[2]w`, cv.name, err)
	}
	if v != cv.value {
		return cv.makeErr(`claim %[1]q does not have the expected value`, cv.name)
	}
	return nil
}

// issuerClaimValueIs creates a Validator that checks if the issuer claim
// matches `value`.
func issuerClaimValueIs(value string) Validator {
	return &claimValueIs{
		name:    IssuerKey,
		value:   value,
		makeErr: jwterrs.IssuerErrorf,
	}
}

// IsRequired creates a Validator that checks if the required claim `name`
// exists in the token
func IsRequired(name string) Validator {
	return isRequired(name)
}

type isRequired string

func (ir isRequired) Validate(_ context.Context, t Token) error {
	name := string(ir)
	if !t.Has(name) {
		return jwterrs.MissingRequiredClaimErrorf(name)
	}
	return nil
}
