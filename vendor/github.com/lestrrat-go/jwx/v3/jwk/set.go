package jwk

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"

	"github.com/lestrrat-go/blackmagic"
	"github.com/lestrrat-go/jwx/v3/internal/json"
	"github.com/lestrrat-go/jwx/v3/internal/pool"
	"github.com/lestrrat-go/jwx/v3/internal/tokens"
)

const keysKey = `keys` // appease linter

// NewSet creates and empty `jwk.Set` object
func NewSet() Set {
	return &set{
		privateParams: make(map[string]any),
	}
}

func (s *set) Set(n string, v any) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if n == keysKey {
		vl, ok := v.([]Key)
		if !ok {
			return fmt.Errorf(`value for field "keys" must be []jwk.Key`)
		}
		s.keys = vl
		return nil
	}

	s.privateParams[n] = v
	return nil
}

func (s *set) Get(name string, dst any) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.privateParams[name]
	if !ok {
		return fmt.Errorf(`field %q not found`, name)
	}
	if err := blackmagic.AssignIfCompatible(dst, v); err != nil {
		return fmt.Errorf(`failed to assign value to dst: %w`, err)
	}
	return nil
}

func (s *set) Key(idx int) (Key, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if idx >= 0 && idx < len(s.keys) {
		return s.keys[idx], true
	}
	return nil, false
}

func (s *set) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.keys)
}

// indexNL is Index(), but without the locking
func (s *set) indexNL(key Key) int {
	for i, k := range s.keys {
		if k == key {
			return i
		}
	}
	return -1
}

func (s *set) Index(key Key) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.indexNL(key)
}

func (s *set) AddKey(key Key) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if reflect.ValueOf(key).IsNil() {
		panic("nil key")
	}

	if i := s.indexNL(key); i > -1 {
		return fmt.Errorf(`(jwk.Set).AddKey: key already exists`)
	}
	s.keys = append(s.keys, key)
	return nil
}

func (s *set) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.privateParams, name)
	return nil
}

func (s *set) RemoveKey(key Key) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, k := range s.keys {
		if k == key {
			switch i {
			case 0:
				s.keys = s.keys[1:]
			case len(s.keys) - 1:
				s.keys = s.keys[:i]
			default:
				s.keys = append(s.keys[:i], s.keys[i+1:]...)
			}
			return nil
		}
	}
	return fmt.Errorf(`(jwk.Set).RemoveKey: specified key does not exist in set`)
}

func (s *set) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.keys = nil
	s.privateParams = make(map[string]any)
	return nil
}

func (s *set) Keys() []string {
	ret := make([]string, len(s.privateParams))
	var i int
	for k := range s.privateParams {
		ret[i] = k
		i++
	}
	return ret
}

func (s *set) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	buf := pool.BytesBuffer().Get()
	defer pool.BytesBuffer().Put(buf)
	enc := json.NewEncoder(buf)

	fields := []string{keysKey}
	for k := range s.privateParams {
		fields = append(fields, k)
	}
	sort.Strings(fields)

	buf.WriteByte(tokens.OpenCurlyBracket)
	for i, field := range fields {
		if i > 0 {
			buf.WriteByte(tokens.Comma)
		}
		fmt.Fprintf(buf, `%q:`, field)
		if field != keysKey {
			if err := enc.Encode(s.privateParams[field]); err != nil {
				return nil, fmt.Errorf(`failed to marshal field %q: %w`, field, err)
			}
		} else {
			buf.WriteByte(tokens.OpenSquareBracket)
			for j, k := range s.keys {
				if j > 0 {
					buf.WriteByte(tokens.Comma)
				}
				if err := enc.Encode(k); err != nil {
					return nil, fmt.Errorf(`failed to marshal key #%d: %w`, i, err)
				}
			}
			buf.WriteByte(tokens.CloseSquareBracket)
		}
	}
	buf.WriteByte(tokens.CloseCurlyBracket)

	ret := make([]byte, buf.Len())
	copy(ret, buf.Bytes())
	return ret, nil
}

func (s *set) UnmarshalJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.privateParams = make(map[string]any)
	s.keys = nil

	var options []ParseOption
	var ignoreParseError bool
	if dc := s.dc; dc != nil {
		if localReg := dc.Registry(); localReg != nil {
			options = append(options, withLocalRegistry(localReg))
		}
		ignoreParseError = dc.IgnoreParseError()
	}

	var sawKeysField bool
	dec := json.NewDecoder(bytes.NewReader(data))
LOOP:
	for {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf(`error reading token: %w`, err)
		}

		switch tok := tok.(type) {
		case json.Delim:
			// Assuming we're doing everything correctly, we should ONLY
			// get either tokens.OpenCurlyBracket or tokens.CloseCurlyBracket here.
			if tok == tokens.CloseCurlyBracket { // End of object
				break LOOP
			} else if tok != tokens.OpenCurlyBracket {
				return fmt.Errorf(`expected '%c' but got '%c'`, tokens.OpenCurlyBracket, tok)
			}
		case string:
			switch tok {
			case "keys":
				sawKeysField = true
				var list []json.RawMessage
				if err := dec.Decode(&list); err != nil {
					return fmt.Errorf(`failed to decode "keys": %w`, err)
				}

				for i, keysrc := range list {
					key, err := ParseKey(keysrc, options...)
					if err != nil {
						if !ignoreParseError {
							return fmt.Errorf(`failed to decode key #%d in "keys": %w`, i, err)
						}
						continue
					}
					s.keys = append(s.keys, key)
				}
			default:
				var v any
				if err := dec.Decode(&v); err != nil {
					return fmt.Errorf(`failed to decode value for key %q: %w`, tok, err)
				}
				s.privateParams[tok] = v
			}
		}
	}

	// This is really silly, but we can only detect the
	// lack of the "keys" field after going through the
	// entire object once
	// Not checking for len(s.keys) == 0, because it could be
	// an empty key set
	if !sawKeysField {
		key, err := ParseKey(data, options...)
		if err != nil {
			return fmt.Errorf(`failed to parse sole key in key set`)
		}
		s.keys = append(s.keys, key)
	}
	return nil
}

func (s *set) LookupKeyID(kid string) (Key, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.Len() {
		key, ok := s.Key(i)
		if !ok {
			return nil, false
		}
		gotkid, ok := key.KeyID()
		if ok && gotkid == kid {
			return key, true
		}
	}
	return nil, false
}

func (s *set) DecodeCtx() DecodeCtx {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dc
}

func (s *set) SetDecodeCtx(dc DecodeCtx) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dc = dc
}

func (s *set) Clone() (Set, error) {
	s2 := &set{}

	s.mu.RLock()
	defer s.mu.RUnlock()

	s2.keys = make([]Key, len(s.keys))
	copy(s2.keys, s.keys)
	return s2, nil
}
