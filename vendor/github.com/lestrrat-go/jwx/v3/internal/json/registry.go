package json

import (
	"fmt"
	"reflect"
	"sync"
)

// CustomDecoder is the interface we expect from RegisterCustomField in jws, jwe, jwk, and jwt packages.
type CustomDecoder interface {
	// Decode takes a JSON encoded byte slice and returns the desired
	// decoded value,which will be used as the value for that field
	// registered through RegisterCustomField
	Decode([]byte) (interface{}, error)
}

// CustomDecodeFunc is a stateless, function-based implementation of CustomDecoder
type CustomDecodeFunc func([]byte) (interface{}, error)

func (fn CustomDecodeFunc) Decode(data []byte) (interface{}, error) {
	return fn(data)
}

type objectTypeDecoder struct {
	typ  reflect.Type
	name string
}

func (dec *objectTypeDecoder) Decode(data []byte) (interface{}, error) {
	ptr := reflect.New(dec.typ).Interface()
	if err := Unmarshal(data, ptr); err != nil {
		return nil, fmt.Errorf(`failed to decode field %s: %w`, dec.name, err)
	}
	return reflect.ValueOf(ptr).Elem().Interface(), nil
}

type Registry struct {
	mu   *sync.RWMutex
	ctrs map[string]CustomDecoder
}

func NewRegistry() *Registry {
	return &Registry{
		mu:   &sync.RWMutex{},
		ctrs: make(map[string]CustomDecoder),
	}
}

func (r *Registry) Register(name string, object interface{}) {
	if object == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.ctrs, name)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if ctr, ok := object.(CustomDecoder); ok {
		r.ctrs[name] = ctr
	} else {
		r.ctrs[name] = &objectTypeDecoder{
			typ:  reflect.TypeOf(object),
			name: name,
		}
	}
}

func (r *Registry) Decode(dec *Decoder, name string) (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if ctr, ok := r.ctrs[name]; ok {
		var raw RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf(`failed to decode field %s: %w`, name, err)
		}
		v, err := ctr.Decode([]byte(raw))
		if err != nil {
			return nil, fmt.Errorf(`failed to decode field %s: %w`, name, err)
		}
		return v, nil
	}

	var decoded interface{}
	if err := dec.Decode(&decoded); err != nil {
		return nil, fmt.Errorf(`failed to decode field %s: %w`, name, err)
	}
	return decoded, nil
}
