package jwe

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lestrrat-go/jwx/v3/internal/base64"
	"github.com/lestrrat-go/jwx/v3/internal/json"
	"github.com/lestrrat-go/jwx/v3/internal/pool"
	"github.com/lestrrat-go/jwx/v3/internal/tokens"
)

// NewRecipient creates a Recipient object
func NewRecipient() Recipient {
	return &stdRecipient{
		headers: NewHeaders(),
	}
}

func (r *stdRecipient) SetHeaders(h Headers) error {
	r.headers = h
	return nil
}

func (r *stdRecipient) SetEncryptedKey(v []byte) error {
	r.encryptedKey = v
	return nil
}

func (r *stdRecipient) Headers() Headers {
	return r.headers
}

func (r *stdRecipient) EncryptedKey() []byte {
	return r.encryptedKey
}

type recipientMarshalProxy struct {
	Headers      Headers `json:"header"`
	EncryptedKey string  `json:"encrypted_key"`
}

func (r *stdRecipient) UnmarshalJSON(buf []byte) error {
	var proxy recipientMarshalProxy
	proxy.Headers = NewHeaders()
	if err := json.Unmarshal(buf, &proxy); err != nil {
		return fmt.Errorf(`failed to unmarshal json into recipient: %w`, err)
	}

	r.headers = proxy.Headers
	decoded, err := base64.DecodeString(proxy.EncryptedKey)
	if err != nil {
		return fmt.Errorf(`failed to decode "encrypted_key": %w`, err)
	}
	r.encryptedKey = decoded
	return nil
}

func (r *stdRecipient) MarshalJSON() ([]byte, error) {
	buf := pool.BytesBuffer().Get()
	defer pool.BytesBuffer().Put(buf)

	buf.WriteString(`{"header":`)
	hdrbuf, err := json.Marshal(r.headers)
	if err != nil {
		return nil, fmt.Errorf(`failed to marshal recipient header: %w`, err)
	}
	buf.Write(hdrbuf)
	buf.WriteString(`,"encrypted_key":"`)
	buf.WriteString(base64.EncodeToString(r.encryptedKey))
	buf.WriteString(`"}`)

	ret := make([]byte, buf.Len())
	copy(ret, buf.Bytes())
	return ret, nil
}

// NewMessage creates a new message
func NewMessage() *Message {
	return &Message{}
}

func (m *Message) AuthenticatedData() []byte {
	return m.authenticatedData
}

func (m *Message) CipherText() []byte {
	return m.cipherText
}

func (m *Message) InitializationVector() []byte {
	return m.initializationVector
}

func (m *Message) Tag() []byte {
	return m.tag
}

func (m *Message) ProtectedHeaders() Headers {
	return m.protectedHeaders
}

func (m *Message) Recipients() []Recipient {
	return m.recipients
}

func (m *Message) UnprotectedHeaders() Headers {
	return m.unprotectedHeaders
}

const (
	AuthenticatedDataKey    = "aad"
	CipherTextKey           = "ciphertext"
	CountKey                = "p2c"
	InitializationVectorKey = "iv"
	ProtectedHeadersKey     = "protected"
	RecipientsKey           = "recipients"
	SaltKey                 = "p2s"
	TagKey                  = "tag"
	UnprotectedHeadersKey   = "unprotected"
	HeadersKey              = "header"
	EncryptedKeyKey         = "encrypted_key"
)

func (m *Message) Set(k string, v any) error {
	switch k {
	case AuthenticatedDataKey:
		buf, ok := v.([]byte)
		if !ok {
			return fmt.Errorf(`invalid value %T for %s key`, v, AuthenticatedDataKey)
		}
		m.authenticatedData = buf
	case CipherTextKey:
		buf, ok := v.([]byte)
		if !ok {
			return fmt.Errorf(`invalid value %T for %s key`, v, CipherTextKey)
		}
		m.cipherText = buf
	case InitializationVectorKey:
		buf, ok := v.([]byte)
		if !ok {
			return fmt.Errorf(`invalid value %T for %s key`, v, InitializationVectorKey)
		}
		m.initializationVector = buf
	case ProtectedHeadersKey:
		cv, ok := v.(Headers)
		if !ok {
			return fmt.Errorf(`invalid value %T for %s key`, v, ProtectedHeadersKey)
		}
		m.protectedHeaders = cv
	case RecipientsKey:
		cv, ok := v.([]Recipient)
		if !ok {
			return fmt.Errorf(`invalid value %T for %s key`, v, RecipientsKey)
		}
		m.recipients = cv
	case TagKey:
		buf, ok := v.([]byte)
		if !ok {
			return fmt.Errorf(`invalid value %T for %s key`, v, TagKey)
		}
		m.tag = buf
	case UnprotectedHeadersKey:
		cv, ok := v.(Headers)
		if !ok {
			return fmt.Errorf(`invalid value %T for %s key`, v, UnprotectedHeadersKey)
		}
		m.unprotectedHeaders = cv
	default:
		if m.unprotectedHeaders == nil {
			m.unprotectedHeaders = NewHeaders()
		}
		return m.unprotectedHeaders.Set(k, v)
	}
	return nil
}

type messageMarshalProxy struct {
	AuthenticatedData    string            `json:"aad,omitempty"`
	CipherText           string            `json:"ciphertext"`
	InitializationVector string            `json:"iv,omitempty"`
	ProtectedHeaders     json.RawMessage   `json:"protected"`
	Recipients           []json.RawMessage `json:"recipients,omitempty"`
	Tag                  string            `json:"tag,omitempty"`
	UnprotectedHeaders   Headers           `json:"unprotected,omitempty"`

	// For flattened structure. Headers is NOT a Headers type,
	// so that we can detect its presence by checking proxy.Headers != nil
	Headers      json.RawMessage `json:"header,omitempty"`
	EncryptedKey string          `json:"encrypted_key,omitempty"`
}

type jsonKV struct {
	Key   string
	Value string
}

func (m *Message) MarshalJSON() ([]byte, error) {
	// This is slightly convoluted, but we need to encode the
	// protected headers, so we do it by hand
	buf := pool.BytesBuffer().Get()
	defer pool.BytesBuffer().Put(buf)
	enc := json.NewEncoder(buf)

	var fields []jsonKV

	if cipherText := m.CipherText(); len(cipherText) > 0 {
		buf.Reset()
		if err := enc.Encode(base64.EncodeToString(cipherText)); err != nil {
			return nil, fmt.Errorf(`failed to encode %s field: %w`, CipherTextKey, err)
		}
		fields = append(fields, jsonKV{
			Key:   CipherTextKey,
			Value: strings.TrimSpace(buf.String()),
		})
	}

	if iv := m.InitializationVector(); len(iv) > 0 {
		buf.Reset()
		if err := enc.Encode(base64.EncodeToString(iv)); err != nil {
			return nil, fmt.Errorf(`failed to encode %s field: %w`, InitializationVectorKey, err)
		}
		fields = append(fields, jsonKV{
			Key:   InitializationVectorKey,
			Value: strings.TrimSpace(buf.String()),
		})
	}

	var encodedProtectedHeaders []byte
	if h := m.ProtectedHeaders(); h != nil {
		v, err := h.Encode()
		if err != nil {
			return nil, fmt.Errorf(`failed to encode protected headers: %w`, err)
		}

		encodedProtectedHeaders = v
		if len(encodedProtectedHeaders) <= 2 { // '{}'
			encodedProtectedHeaders = nil
		} else {
			fields = append(fields, jsonKV{
				Key:   ProtectedHeadersKey,
				Value: fmt.Sprintf("%q", encodedProtectedHeaders),
			})
		}
	}

	if aad := m.AuthenticatedData(); len(aad) > 0 {
		aad = base64.Encode(aad)
		if encodedProtectedHeaders != nil {
			tmp := append(encodedProtectedHeaders, tokens.Period)
			aad = append(tmp, aad...)
		}

		buf.Reset()
		if err := enc.Encode(aad); err != nil {
			return nil, fmt.Errorf(`failed to encode %s field: %w`, AuthenticatedDataKey, err)
		}
		fields = append(fields, jsonKV{
			Key:   AuthenticatedDataKey,
			Value: strings.TrimSpace(buf.String()),
		})
	}

	if recipients := m.Recipients(); len(recipients) > 0 {
		if len(recipients) == 1 { // Use flattened format
			if hdrs := recipients[0].Headers(); hdrs != nil {
				buf.Reset()
				if err := enc.Encode(hdrs); err != nil {
					return nil, fmt.Errorf(`failed to encode %s field: %w`, HeadersKey, err)
				}
				fields = append(fields, jsonKV{
					Key:   HeadersKey,
					Value: strings.TrimSpace(buf.String()),
				})
			}

			if ek := recipients[0].EncryptedKey(); len(ek) > 0 {
				buf.Reset()
				if err := enc.Encode(base64.EncodeToString(ek)); err != nil {
					return nil, fmt.Errorf(`failed to encode %s field: %w`, EncryptedKeyKey, err)
				}
				fields = append(fields, jsonKV{
					Key:   EncryptedKeyKey,
					Value: strings.TrimSpace(buf.String()),
				})
			}
		} else {
			buf.Reset()
			if err := enc.Encode(recipients); err != nil {
				return nil, fmt.Errorf(`failed to encode %s field: %w`, RecipientsKey, err)
			}
			fields = append(fields, jsonKV{
				Key:   RecipientsKey,
				Value: strings.TrimSpace(buf.String()),
			})
		}
	}

	if tag := m.Tag(); len(tag) > 0 {
		buf.Reset()
		if err := enc.Encode(base64.EncodeToString(tag)); err != nil {
			return nil, fmt.Errorf(`failed to encode %s field: %w`, TagKey, err)
		}
		fields = append(fields, jsonKV{
			Key:   TagKey,
			Value: strings.TrimSpace(buf.String()),
		})
	}

	if h := m.UnprotectedHeaders(); h != nil {
		unprotected, err := json.Marshal(h)
		if err != nil {
			return nil, fmt.Errorf(`failed to encode unprotected headers: %w`, err)
		}

		if len(unprotected) > 2 {
			fields = append(fields, jsonKV{
				Key:   UnprotectedHeadersKey,
				Value: fmt.Sprintf("%q", unprotected),
			})
		}
	}

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Key < fields[j].Key
	})
	buf.Reset()
	fmt.Fprintf(buf, `{`)
	for i, kv := range fields {
		if i > 0 {
			fmt.Fprintf(buf, `,`)
		}
		fmt.Fprintf(buf, `%q:%s`, kv.Key, kv.Value)
	}
	fmt.Fprintf(buf, `}`)

	ret := make([]byte, buf.Len())
	copy(ret, buf.Bytes())
	return ret, nil
}

func (m *Message) UnmarshalJSON(buf []byte) error {
	var proxy messageMarshalProxy
	proxy.UnprotectedHeaders = NewHeaders()

	if err := json.Unmarshal(buf, &proxy); err != nil {
		return fmt.Errorf(`failed to unmashal JSON into message: %w`, err)
	}

	// Get the string value
	var protectedHeadersStr string
	if err := json.Unmarshal(proxy.ProtectedHeaders, &protectedHeadersStr); err != nil {
		return fmt.Errorf(`failed to decode protected headers (1): %w`, err)
	}

	// It's now in _quoted_ base64 string. Decode it
	protectedHeadersRaw, err := base64.DecodeString(protectedHeadersStr)
	if err != nil {
		return fmt.Errorf(`failed to base64 decoded protected headers buffer: %w`, err)
	}

	h := NewHeaders()
	if err := json.Unmarshal(protectedHeadersRaw, h); err != nil {
		return fmt.Errorf(`failed to decode protected headers (2): %w`, err)
	}

	// if this were a flattened message, we would see a "header" and "ciphertext"
	// field. TODO: do both of these conditions need to meet, or just one?
	if proxy.Headers != nil || len(proxy.EncryptedKey) > 0 {
		recipient := NewRecipient()
		hdrs := NewHeaders()
		if err := json.Unmarshal(proxy.Headers, hdrs); err != nil {
			return fmt.Errorf(`failed to decode headers field: %w`, err)
		}

		if err := recipient.SetHeaders(hdrs); err != nil {
			return fmt.Errorf(`failed to set new headers: %w`, err)
		}

		if v := proxy.EncryptedKey; len(v) > 0 {
			buf, err := base64.DecodeString(v)
			if err != nil {
				return fmt.Errorf(`failed to decode encrypted key: %w`, err)
			}
			if err := recipient.SetEncryptedKey(buf); err != nil {
				return fmt.Errorf(`failed to set encrypted key: %w`, err)
			}
		}

		m.recipients = append(m.recipients, recipient)
	} else {
		for i, recipientbuf := range proxy.Recipients {
			recipient := NewRecipient()
			if err := json.Unmarshal(recipientbuf, recipient); err != nil {
				return fmt.Errorf(`failed to decode recipient at index %d: %w`, i, err)
			}

			m.recipients = append(m.recipients, recipient)
		}
	}

	if src := proxy.AuthenticatedData; len(src) > 0 {
		v, err := base64.DecodeString(src)
		if err != nil {
			return fmt.Errorf(`failed to decode "aad": %w`, err)
		}
		m.authenticatedData = v
	}

	if src := proxy.CipherText; len(src) > 0 {
		v, err := base64.DecodeString(src)
		if err != nil {
			return fmt.Errorf(`failed to decode "ciphertext": %w`, err)
		}
		m.cipherText = v
	}

	if src := proxy.InitializationVector; len(src) > 0 {
		v, err := base64.DecodeString(src)
		if err != nil {
			return fmt.Errorf(`failed to decode "iv": %w`, err)
		}
		m.initializationVector = v
	}

	if src := proxy.Tag; len(src) > 0 {
		v, err := base64.DecodeString(src)
		if err != nil {
			return fmt.Errorf(`failed to decode "tag": %w`, err)
		}
		m.tag = v
	}

	m.protectedHeaders = h
	if m.storeProtectedHeaders {
		// this is later used for decryption
		m.rawProtectedHeaders = base64.Encode(protectedHeadersRaw)
	}

	if iz, ok := proxy.UnprotectedHeaders.(isZeroer); ok {
		if !iz.isZero() {
			m.unprotectedHeaders = proxy.UnprotectedHeaders
		}
	}

	if len(m.recipients) == 0 {
		if err := m.makeDummyRecipient(proxy.EncryptedKey, m.protectedHeaders); err != nil {
			return fmt.Errorf(`failed to setup recipient: %w`, err)
		}
	}

	return nil
}

func (m *Message) makeDummyRecipient(enckeybuf string, protected Headers) error {
	// Recipients in this case should not contain the content encryption key,
	// so move that out
	hdrs, err := protected.Clone()
	if err != nil {
		return fmt.Errorf(`failed to clone headers: %w`, err)
	}

	if err := hdrs.Remove(ContentEncryptionKey); err != nil {
		return fmt.Errorf(`failed to remove %#v from public header: %w`, ContentEncryptionKey, err)
	}

	enckey, err := base64.DecodeString(enckeybuf)
	if err != nil {
		return fmt.Errorf(`failed to decode encrypted key: %w`, err)
	}

	if err := m.Set(RecipientsKey, []Recipient{
		&stdRecipient{
			headers:      hdrs,
			encryptedKey: enckey,
		},
	}); err != nil {
		return fmt.Errorf(`failed to set %s: %w`, RecipientsKey, err)
	}
	return nil
}

// Compact generates a JWE message in compact serialization format from a
// `*jwe.Message` object. The object contain exactly one recipient, or
// an error is returned.
//
// This function currently does not take any options, but the function
// signature contains `options` for possible future expansion of the API
func Compact(m *Message, _ ...CompactOption) ([]byte, error) {
	if len(m.recipients) != 1 {
		return nil, fmt.Errorf(`wrong number of recipients for compact serialization`)
	}

	recipient := m.recipients[0]

	// The protected header must be a merge between the message-wide
	// protected header AND the recipient header

	// There's something wrong if m.protectedHeaders is nil, but
	// it could happen
	if m.protectedHeaders == nil {
		return nil, fmt.Errorf(`invalid protected header`)
	}

	hcopy, err := m.protectedHeaders.Clone()
	if err != nil {
		return nil, fmt.Errorf(`failed to copy protected header: %w`, err)
	}
	hcopy, err = hcopy.Merge(m.unprotectedHeaders)
	if err != nil {
		return nil, fmt.Errorf(`failed to merge unprotected header: %w`, err)
	}
	hcopy, err = hcopy.Merge(recipient.Headers())
	if err != nil {
		return nil, fmt.Errorf(`failed to merge recipient header: %w`, err)
	}

	protected, err := hcopy.Encode()
	if err != nil {
		return nil, fmt.Errorf(`failed to encode header: %w`, err)
	}

	encryptedKey := base64.Encode(recipient.EncryptedKey())
	iv := base64.Encode(m.initializationVector)
	cipher := base64.Encode(m.cipherText)
	tag := base64.Encode(m.tag)

	buf := pool.BytesBuffer().Get()
	defer pool.BytesBuffer().Put(buf)

	buf.Grow(len(protected) + len(encryptedKey) + len(iv) + len(cipher) + len(tag) + 4)
	buf.Write(protected)
	buf.WriteByte(tokens.Period)
	buf.Write(encryptedKey)
	buf.WriteByte(tokens.Period)
	buf.Write(iv)
	buf.WriteByte(tokens.Period)
	buf.Write(cipher)
	buf.WriteByte(tokens.Period)
	buf.Write(tag)

	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result, nil
}
