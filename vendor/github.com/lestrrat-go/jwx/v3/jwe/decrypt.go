package jwe

import (
	"fmt"

	"github.com/lestrrat-go/jwx/v3/internal/tokens"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwe/internal/content_crypt"
	"github.com/lestrrat-go/jwx/v3/jwe/jwebb"
)

// decrypter is responsible for taking various components to decrypt a message.
// its operation is not concurrency safe. You must provide locking yourself
//
//nolint:govet
type decrypter struct {
	aad         []byte
	apu         []byte
	apv         []byte
	cek         *[]byte
	computedAad []byte
	iv          []byte
	keyiv       []byte
	keysalt     []byte
	keytag      []byte
	tag         []byte
	privkey     any
	pubkey      any
	ctalg       jwa.ContentEncryptionAlgorithm
	keyalg      jwa.KeyEncryptionAlgorithm
	cipher      content_crypt.Cipher
	keycount    int
}

// newDecrypter Creates a new Decrypter instance. You must supply the
// rest of parameters via their respective setter methods before
// calling Decrypt().
//
// privkey must be a private key in its "raw" format (i.e. something like
// *rsa.PrivateKey, instead of jwk.Key)
//
// You should consider this object immutable once you assign values to it.
func newDecrypter(keyalg jwa.KeyEncryptionAlgorithm, ctalg jwa.ContentEncryptionAlgorithm, privkey any) *decrypter {
	return &decrypter{
		ctalg:   ctalg,
		keyalg:  keyalg,
		privkey: privkey,
	}
}

func (d *decrypter) AgreementPartyUInfo(apu []byte) *decrypter {
	d.apu = apu
	return d
}

func (d *decrypter) AgreementPartyVInfo(apv []byte) *decrypter {
	d.apv = apv
	return d
}

func (d *decrypter) AuthenticatedData(aad []byte) *decrypter {
	d.aad = aad
	return d
}

func (d *decrypter) ComputedAuthenticatedData(aad []byte) *decrypter {
	d.computedAad = aad
	return d
}

func (d *decrypter) ContentEncryptionAlgorithm(ctalg jwa.ContentEncryptionAlgorithm) *decrypter {
	d.ctalg = ctalg
	return d
}

func (d *decrypter) InitializationVector(iv []byte) *decrypter {
	d.iv = iv
	return d
}

func (d *decrypter) KeyCount(keycount int) *decrypter {
	d.keycount = keycount
	return d
}

func (d *decrypter) KeyInitializationVector(keyiv []byte) *decrypter {
	d.keyiv = keyiv
	return d
}

func (d *decrypter) KeySalt(keysalt []byte) *decrypter {
	d.keysalt = keysalt
	return d
}

func (d *decrypter) KeyTag(keytag []byte) *decrypter {
	d.keytag = keytag
	return d
}

// PublicKey sets the public key to be used in decoding EC based encryptions.
// The key must be in its "raw" format (i.e. *ecdsa.PublicKey, instead of jwk.Key)
func (d *decrypter) PublicKey(pubkey any) *decrypter {
	d.pubkey = pubkey
	return d
}

func (d *decrypter) Tag(tag []byte) *decrypter {
	d.tag = tag
	return d
}

func (d *decrypter) CEK(ptr *[]byte) *decrypter {
	d.cek = ptr
	return d
}

func (d *decrypter) ContentCipher() (content_crypt.Cipher, error) {
	if d.cipher == nil {
		cipher, err := jwebb.CreateContentCipher(d.ctalg.String())
		if err != nil {
			return nil, err
		}
		d.cipher = cipher
	}

	return d.cipher, nil
}

func (d *decrypter) Decrypt(recipient Recipient, ciphertext []byte, msg *Message) (plaintext []byte, err error) {
	cek, keyerr := d.DecryptKey(recipient, msg)
	if keyerr != nil {
		err = fmt.Errorf(`failed to decrypt key: %w`, keyerr)
		return
	}

	cipher, ciphererr := d.ContentCipher()
	if ciphererr != nil {
		err = fmt.Errorf(`failed to fetch content crypt cipher: %w`, ciphererr)
		return
	}

	computedAad := d.computedAad
	if d.aad != nil {
		computedAad = append(append(computedAad, tokens.Period), d.aad...)
	}

	plaintext, err = cipher.Decrypt(cek, d.iv, ciphertext, d.tag, computedAad)
	if err != nil {
		err = fmt.Errorf(`failed to decrypt payload: %w`, err)
		return
	}

	if d.cek != nil {
		*d.cek = cek
	}
	return plaintext, nil
}

func (d *decrypter) DecryptKey(recipient Recipient, msg *Message) (cek []byte, err error) {
	recipientKey := recipient.EncryptedKey()
	if kd, ok := d.privkey.(KeyDecrypter); ok {
		return kd.DecryptKey(d.keyalg, recipientKey, recipient, msg)
	}

	if jwebb.IsDirect(d.keyalg.String()) {
		cek, ok := d.privkey.([]byte)
		if !ok {
			return nil, fmt.Errorf("decrypt key: []byte is required as the key for %s (got %T)", d.keyalg, d.privkey)
		}
		return jwebb.KeyDecryptDirect(recipientKey, recipientKey, d.keyalg.String(), cek)
	}

	if jwebb.IsPBES2(d.keyalg.String()) {
		password, ok := d.privkey.([]byte)
		if !ok {
			return nil, fmt.Errorf("decrypt key: []byte is required as the password for %s (got %T)", d.keyalg, d.privkey)
		}
		salt := []byte(d.keyalg.String())
		salt = append(salt, byte(0))
		salt = append(salt, d.keysalt...)
		return jwebb.KeyDecryptPBES2(recipientKey, recipientKey, d.keyalg.String(), password, salt, d.keycount)
	}

	if jwebb.IsAESGCMKW(d.keyalg.String()) {
		sharedkey, ok := d.privkey.([]byte)
		if !ok {
			return nil, fmt.Errorf("decrypt key: []byte is required as the key for %s (got %T)", d.keyalg, d.privkey)
		}
		return jwebb.KeyDecryptAESGCMKW(recipientKey, recipientKey, d.keyalg.String(), sharedkey, d.keyiv, d.keytag)
	}

	if jwebb.IsECDHES(d.keyalg.String()) {
		alg, keysize, keywrap, err := jwebb.KeyEncryptionECDHESKeySize(d.keyalg.String(), d.ctalg.String())
		if err != nil {
			return nil, fmt.Errorf(`failed to determine ECDH-ES key size: %w`, err)
		}

		if !keywrap {
			return jwebb.KeyDecryptECDHES(recipientKey, cek, alg, d.apu, d.apv, d.privkey, d.pubkey, keysize)
		}
		return jwebb.KeyDecryptECDHESKeyWrap(recipientKey, recipientKey, d.keyalg.String(), d.apu, d.apv, d.privkey, d.pubkey, keysize)
	}

	if jwebb.IsRSA15(d.keyalg.String()) {
		cipher, err := d.ContentCipher()
		if err != nil {
			return nil, fmt.Errorf(`failed to fetch content crypt cipher: %w`, err)
		}
		keysize := cipher.KeySize() / 2
		return jwebb.KeyDecryptRSA15(recipientKey, recipientKey, d.privkey, keysize)
	}

	if jwebb.IsRSAOAEP(d.keyalg.String()) {
		return jwebb.KeyDecryptRSAOAEP(recipientKey, recipientKey, d.keyalg.String(), d.privkey)
	}

	if jwebb.IsAESKW(d.keyalg.String()) {
		sharedkey, ok := d.privkey.([]byte)
		if !ok {
			return nil, fmt.Errorf("[]byte is required as the key to decrypt %s", d.keyalg.String())
		}
		return jwebb.KeyDecryptAESKW(recipientKey, recipientKey, d.keyalg.String(), sharedkey)
	}

	return nil, fmt.Errorf(`unsupported algorithm for key decryption (%s)`, d.keyalg)
}
