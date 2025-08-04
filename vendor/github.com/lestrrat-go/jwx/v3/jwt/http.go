package jwt

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/lestrrat-go/jwx/v3/internal/pool"
	"github.com/lestrrat-go/jwx/v3/internal/tokens"
)

// ParseCookie parses a JWT stored in a http.Cookie with the given name.
// If the specified cookie is not found, http.ErrNoCookie is returned.
func ParseCookie(req *http.Request, name string, options ...ParseOption) (Token, error) {
	var dst **http.Cookie
	for _, option := range options {
		switch option.Ident() {
		case identCookie{}:
			if err := option.Value(&dst); err != nil {
				return nil, fmt.Errorf(`jws.ParseCookie: value to option WithCookie must be **http.Cookie: %w`, err)
			}
		}
	}

	cookie, err := req.Cookie(name)
	if err != nil {
		return nil, err
	}
	tok, err := ParseString(cookie.Value, options...)
	if err != nil {
		return nil, fmt.Errorf(`jws.ParseCookie: failed to parse token stored in cookie: %w`, err)
	}

	if dst != nil {
		*dst = cookie
	}
	return tok, nil
}

// ParseHeader parses a JWT stored in a http.Header.
//
// For the header "Authorization", it will strip the prefix "Bearer " and will
// treat the remaining value as a JWT.
func ParseHeader(hdr http.Header, name string, options ...ParseOption) (Token, error) {
	key := http.CanonicalHeaderKey(name)
	v := strings.TrimSpace(hdr.Get(key))
	if v == "" {
		return nil, fmt.Errorf(`empty header (%s)`, key)
	}

	if key == "Authorization" {
		// Authorization header is an exception. We strip the "Bearer " from
		// the prefix
		v = strings.TrimSpace(strings.TrimPrefix(v, "Bearer"))
	}

	tok, err := ParseString(v, options...)
	if err != nil {
		return nil, fmt.Errorf(`failed to parse token stored in header (%s): %w`, key, err)
	}
	return tok, nil
}

// ParseForm parses a JWT stored in a url.Value.
func ParseForm(values url.Values, name string, options ...ParseOption) (Token, error) {
	v := strings.TrimSpace(values.Get(name))
	if v == "" {
		return nil, fmt.Errorf(`empty value (%s)`, name)
	}

	return ParseString(v, options...)
}

// ParseRequest searches a http.Request object for a JWT token.
//
// Specifying WithHeaderKey() will tell it to search under a specific
// header key. Specifying WithFormKey() will tell it to search under
// a specific form field.
//
// If none of jwt.WithHeaderKey()/jwt.WithCookieKey()/jwt.WithFormKey() is
// used, "Authorization" header will be searched. If any of these options
// are specified, you must explicitly re-enable searching for "Authorization" header
// if you also want to search for it.
//
//	# searches for "Authorization"
//	jwt.ParseRequest(req)
//
//	# searches for "x-my-token" ONLY.
//	jwt.ParseRequest(req, jwt.WithHeaderKey("x-my-token"))
//
//	# searches for "Authorization" AND "x-my-token"
//	jwt.ParseRequest(req, jwt.WithHeaderKey("Authorization"), jwt.WithHeaderKey("x-my-token"))
//
// Cookies are searched using (http.Request).Cookie(). If you have multiple
// cookies with the same name, and you want to search for a specific one that
// (http.Request).Cookie() would not return, you will need to implement your
// own logic to extract the cookie and use jwt.ParseString().
func ParseRequest(req *http.Request, options ...ParseOption) (Token, error) {
	var hdrkeys []string
	var formkeys []string
	var cookiekeys []string
	var parseOptions []ParseOption
	for _, option := range options {
		switch option.Ident() {
		case identHeaderKey{}:
			var v string
			if err := option.Value(&v); err != nil {
				return nil, fmt.Errorf(`jws.ParseRequest: value to option WithHeaderKey must be string: %w`, err)
			}
			hdrkeys = append(hdrkeys, v)
		case identFormKey{}:
			var v string
			if err := option.Value(&v); err != nil {
				return nil, fmt.Errorf(`jws.ParseRequest: value to option WithFormKey must be string: %w`, err)
			}
			formkeys = append(formkeys, v)
		case identCookieKey{}:
			var v string
			if err := option.Value(&v); err != nil {
				return nil, fmt.Errorf(`jws.ParseRequest: value to option WithCookieKey must be string: %w`, err)
			}
			cookiekeys = append(cookiekeys, v)
		default:
			parseOptions = append(parseOptions, option)
		}
	}

	if len(hdrkeys) == 0 && len(formkeys) == 0 && len(cookiekeys) == 0 {
		hdrkeys = append(hdrkeys, "Authorization")
	}

	mhdrs := pool.KeyToErrorMap().Get()
	defer pool.KeyToErrorMap().Put(mhdrs)
	mfrms := pool.KeyToErrorMap().Get()
	defer pool.KeyToErrorMap().Put(mfrms)
	mcookies := pool.KeyToErrorMap().Get()
	defer pool.KeyToErrorMap().Put(mcookies)

	for _, hdrkey := range hdrkeys {
		// Check presence via a direct map lookup
		if _, ok := req.Header[http.CanonicalHeaderKey(hdrkey)]; !ok {
			// if non-existent, not error
			continue
		}

		tok, err := ParseHeader(req.Header, hdrkey, parseOptions...)
		if err != nil {
			mhdrs[hdrkey] = err
			continue
		}
		return tok, nil
	}

	for _, name := range cookiekeys {
		tok, err := ParseCookie(req, name, parseOptions...)
		if err != nil {
			if err == http.ErrNoCookie {
				// not fatal
				mcookies[name] = err
			}
			continue
		}
		return tok, nil
	}

	if cl := req.ContentLength; cl > 0 {
		if err := req.ParseForm(); err != nil {
			return nil, fmt.Errorf(`failed to parse form: %w`, err)
		}
	}

	for _, formkey := range formkeys {
		// Check presence via a direct map lookup
		if _, ok := req.Form[formkey]; !ok {
			// if non-existent, not error
			continue
		}

		tok, err := ParseForm(req.Form, formkey, parseOptions...)
		if err != nil {
			mfrms[formkey] = err
			continue
		}
		return tok, nil
	}

	// Everything below is a prelude to error reporting.
	var triedHdrs strings.Builder
	for i, hdrkey := range hdrkeys {
		if i > 0 {
			triedHdrs.WriteString(", ")
		}
		triedHdrs.WriteString(strconv.Quote(hdrkey))
	}

	var triedForms strings.Builder
	for i, formkey := range formkeys {
		if i > 0 {
			triedForms.WriteString(", ")
		}
		triedForms.WriteString(strconv.Quote(formkey))
	}

	var triedCookies strings.Builder
	for i, cookiekey := range cookiekeys {
		if i > 0 {
			triedCookies.WriteString(", ")
		}
		triedCookies.WriteString(strconv.Quote(cookiekey))
	}

	var b strings.Builder
	b.WriteString(`failed to find a valid token in any location of the request (tried: `)
	olen := b.Len()
	if triedHdrs.Len() > 0 {
		b.WriteString(`header keys: [`)
		b.WriteString(triedHdrs.String())
		b.WriteByte(tokens.CloseSquareBracket)
	}
	if triedForms.Len() > 0 {
		if b.Len() > olen {
			b.WriteString(", ")
		}
		b.WriteString("form keys: [")
		b.WriteString(triedForms.String())
		b.WriteByte(tokens.CloseSquareBracket)
	}

	if triedCookies.Len() > 0 {
		if b.Len() > olen {
			b.WriteString(", ")
		}
		b.WriteString("cookie keys: [")
		b.WriteString(triedCookies.String())
		b.WriteByte(tokens.CloseSquareBracket)
	}
	b.WriteByte(')')

	lmhdrs := len(mhdrs)
	lmfrms := len(mfrms)
	lmcookies := len(mcookies)
	var errors []any
	if lmhdrs > 0 || lmfrms > 0 || lmcookies > 0 {
		b.WriteString(". Additionally, errors were encountered during attempts to verify using:")

		if lmhdrs > 0 {
			b.WriteString(" headers: (")
			count := 0
			for hdrkey, err := range mhdrs {
				if count > 0 {
					b.WriteString(", ")
				}
				b.WriteString("[header key: ")
				b.WriteString(strconv.Quote(hdrkey))
				b.WriteString(", error: %w]")
				errors = append(errors, err)
				count++
			}
			b.WriteString(")")
		}

		if lmcookies > 0 {
			count := 0
			b.WriteString(" cookies: (")
			for cookiekey, err := range mcookies {
				if count > 0 {
					b.WriteString(", ")
				}
				b.WriteString("[cookie key: ")
				b.WriteString(strconv.Quote(cookiekey))
				b.WriteString(", error: %w]")
				errors = append(errors, err)
				count++
			}
		}

		if lmfrms > 0 {
			count := 0
			b.WriteString(" forms: (")
			for formkey, err := range mfrms {
				if count > 0 {
					b.WriteString(", ")
				}
				b.WriteString("[form key: ")
				b.WriteString(strconv.Quote(formkey))
				b.WriteString(", error: %w]")
				errors = append(errors, err)
				count++
			}
		}
	}
	return nil, fmt.Errorf(b.String(), errors...)
}
