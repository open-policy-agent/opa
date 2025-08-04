package jwebb

import (
	"github.com/lestrrat-go/jwx/v3/internal/tokens"
)

// IsECDHES checks if the algorithm is an ECDH-ES based algorithm
func IsECDHES(alg string) bool {
	switch alg {
	case tokens.ECDH_ES, tokens.ECDH_ES_A128KW, tokens.ECDH_ES_A192KW, tokens.ECDH_ES_A256KW:
		return true
	default:
		return false
	}
}

// IsRSA15 checks if the algorithm is RSA1_5
func IsRSA15(alg string) bool {
	return alg == tokens.RSA1_5
}

// IsRSAOAEP checks if the algorithm is an RSA-OAEP based algorithm
func IsRSAOAEP(alg string) bool {
	switch alg {
	case tokens.RSA_OAEP, tokens.RSA_OAEP_256, tokens.RSA_OAEP_384, tokens.RSA_OAEP_512:
		return true
	default:
		return false
	}
}

// IsAESKW checks if the algorithm is an AES key wrap algorithm
func IsAESKW(alg string) bool {
	switch alg {
	case tokens.A128KW, tokens.A192KW, tokens.A256KW:
		return true
	default:
		return false
	}
}

// IsAESGCMKW checks if the algorithm is an AES-GCM key wrap algorithm
func IsAESGCMKW(alg string) bool {
	switch alg {
	case tokens.A128GCMKW, tokens.A192GCMKW, tokens.A256GCMKW:
		return true
	default:
		return false
	}
}

// IsPBES2 checks if the algorithm is a PBES2 based algorithm
func IsPBES2(alg string) bool {
	switch alg {
	case tokens.PBES2_HS256_A128KW, tokens.PBES2_HS384_A192KW, tokens.PBES2_HS512_A256KW:
		return true
	default:
		return false
	}
}

// IsDirect checks if the algorithm is direct encryption
func IsDirect(alg string) bool {
	return alg == tokens.DIRECT
}

// IsSymmetric checks if the algorithm is a symmetric key encryption algorithm
func IsSymmetric(alg string) bool {
	return IsAESKW(alg) || IsAESGCMKW(alg) || IsPBES2(alg) || IsDirect(alg)
}
