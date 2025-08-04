package keygen

// ByteKey is a generated key that only has the key's byte buffer
// as its instance data. If a key needs to do more, such as providing
// values to be set in a JWE header, that key type wraps a ByteKey
type ByteKey []byte

// ByteWithECPublicKey holds the EC private key that generated
// the key along with the key itself. This is required to set the
// proper values in the JWE headers
type ByteWithECPublicKey struct {
	ByteKey

	PublicKey any
}

type ByteWithIVAndTag struct {
	ByteKey

	IV  []byte
	Tag []byte
}

type ByteWithSaltAndCount struct {
	ByteKey

	Salt  []byte
	Count int
}

// ByteSource is an interface for things that return a byte sequence.
// This is used for KeyGenerator so that the result of computations can
// carry more than just the generate byte sequence.
type ByteSource interface {
	Bytes() []byte
}

type Setter interface {
	Set(string, any) error
}
