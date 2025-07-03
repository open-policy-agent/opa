package tokens

const (
	CloseCurlyBracket  = '}'
	CloseSquareBracket = ']'
	Colon              = ':'
	Comma              = ','
	DoubleQuote        = '"'
	OpenCurlyBracket   = '{'
	OpenSquareBracket  = '['
	Period             = '.'
)

// Cryptographic key sizes
const (
	KeySize16 = 16
	KeySize24 = 24
	KeySize32 = 32
	KeySize48 = 48 // A192CBC_HS384 key size
	KeySize64 = 64 // A256CBC_HS512 key size
)

// Bit/byte conversion factors
const (
	BitsPerByte = 8
	BytesPerBit = 1.0 / 8
)

// Key wrapping constants
const (
	KeywrapChunkLen  = 8
	KeywrapRounds    = 6 // RFC 3394 key wrap rounds
	KeywrapBlockSize = 8 // Key wrap block size in bytes
)

// AES-GCM constants
const (
	GCMIVSize  = 12 // GCM IV size in bytes (96 bits)
	GCMTagSize = 16 // GCM tag size in bytes (128 bits)
)

// PBES2 constants
const (
	PBES2DefaultIterations = 10000 // Default PBKDF2 iteration count
	PBES2NullByteSeparator = 0     // Null byte separator for PBES2
)

// RSA key generation constants
const (
	RSAKeyGenMultiplier = 2 // RSA key generation size multiplier
)
