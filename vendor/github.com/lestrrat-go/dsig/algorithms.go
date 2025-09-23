package dsig

// This file defines verbose algorithm name constants that can be mapped to by
// different standards (RFC7518, FIDO, etc.) for interoperability.
//
// The algorithm names are intentionally verbose to avoid any ambiguity about
// the exact cryptographic operations being performed.

const (
	// HMAC signature algorithms
	// These use Hash-based Message Authentication Code with specified hash functions
	HMACWithSHA256 = "HMAC_WITH_SHA256"
	HMACWithSHA384 = "HMAC_WITH_SHA384"
	HMACWithSHA512 = "HMAC_WITH_SHA512"

	// RSA signature algorithms with PKCS#1 v1.5 padding
	// These use RSA signatures with PKCS#1 v1.5 padding and specified hash functions
	RSAPKCS1v15WithSHA256 = "RSA_PKCS1v15_WITH_SHA256"
	RSAPKCS1v15WithSHA384 = "RSA_PKCS1v15_WITH_SHA384"
	RSAPKCS1v15WithSHA512 = "RSA_PKCS1v15_WITH_SHA512"

	// RSA signature algorithms with PSS padding
	// These use RSA signatures with Probabilistic Signature Scheme (PSS) padding
	RSAPSSWithSHA256 = "RSA_PSS_WITH_SHA256"
	RSAPSSWithSHA384 = "RSA_PSS_WITH_SHA384"
	RSAPSSWithSHA512 = "RSA_PSS_WITH_SHA512"

	// ECDSA signature algorithms
	// These use Elliptic Curve Digital Signature Algorithm with specified curves and hash functions
	ECDSAWithP256AndSHA256 = "ECDSA_WITH_P256_AND_SHA256"
	ECDSAWithP384AndSHA384 = "ECDSA_WITH_P384_AND_SHA384"
	ECDSAWithP521AndSHA512 = "ECDSA_WITH_P521_AND_SHA512"

	// EdDSA signature algorithms
	// These use Edwards-curve Digital Signature Algorithm (supports Ed25519 and Ed448)
	EdDSA = "EDDSA"
)