// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rest

import (
	"cmp"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

const (
	defaultMinTLSVersion = tls.VersionTLS12
)

// DefaultTLSConfig defines standard TLS configurations based on the Config
func DefaultTLSConfig(c Config) (*tls.Config, error) {
	t := &tls.Config{
		MinVersion: defaultMinTLSVersion,
	}
	url, err := url.Parse(c.URL)
	if err != nil {
		return nil, err
	}
	if url.Scheme == "https" {
		t.InsecureSkipVerify = c.AllowInsecureTLS
	}

	if c.TLS != nil && c.TLS.CACert != "" {
		caCert, err := os.ReadFile(c.TLS.CACert)
		if err != nil {
			return nil, err
		}

		var rootCAs *x509.CertPool
		if c.TLS.SystemCARequired {
			rootCAs, err = x509.SystemCertPool()
			if err != nil {
				return nil, err
			}
		} else {
			rootCAs = x509.NewCertPool()
		}

		ok := rootCAs.AppendCertsFromPEM(caCert)
		if !ok {
			return nil, errors.New("unable to parse and append CA certificate to certificate pool")
		}
		t.RootCAs = rootCAs
	}

	return t, nil
}

// DefaultRoundTripperClient is a reasonable set of defaults for HTTP auth plugins
func DefaultRoundTripperClient(t *tls.Config, timeout int64) *http.Client {
	// Ensure we use a http.Transport with proper settings: the zero values are not
	// a good choice, as they cause leaking connections:
	// https://github.com/golang/go/issues/19620

	// copy, we don't want to alter the default client's Transport
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ResponseHeaderTimeout = time.Duration(timeout) * time.Second
	tr.TLSClientConfig = t

	c := *http.DefaultClient
	c.Transport = tr
	return &c
}

type serverTLSConfig struct {
	CACert           string `json:"ca_cert,omitempty"`
	SystemCARequired bool   `json:"system_ca_required,omitempty"`
}

// clientTLSAuthPlugin represents authentication via client certificate on a TLS connection
type clientTLSAuthPlugin struct {
	Cert                      string `json:"cert"`
	PrivateKey                string `json:"private_key"`
	PrivateKeyPassphrase      string `json:"private_key_passphrase,omitempty"`
	CACert                    string `json:"ca_cert,omitempty"`            // Deprecated: Use `services[_].tls.ca_cert` instead
	SystemCARequired          bool   `json:"system_ca_required,omitempty"` // Deprecated: Use `services[_].tls.system_ca_required` instead
	CertRereadIntervalSeconds *int64 `json:"cert_reread_interval_seconds,omitempty"`

	mu           sync.RWMutex
	cachedCert   *tls.Certificate
	certFileHash [32]byte
	keyFileHash  [32]byte
	lastLoadTime time.Time
}

func (ap *clientTLSAuthPlugin) loadCertificate() (*tls.Certificate, error) {
	rereadIntervalSeconds := int64(0)
	if ap.CertRereadIntervalSeconds != nil {
		rereadIntervalSeconds = *ap.CertRereadIntervalSeconds
	}

	ap.mu.RLock()
	if ap.cachedCert != nil && rereadIntervalSeconds > 0 {
		timeSinceLastLoad := time.Since(ap.lastLoadTime).Seconds()
		if timeSinceLastLoad < float64(rereadIntervalSeconds) {
			cert := ap.cachedCert
			ap.mu.RUnlock()
			return cert, nil
		}
	}
	ap.mu.RUnlock()

	certPEMBlock, err := os.ReadFile(ap.Cert)
	if err != nil {
		return nil, fmt.Errorf("failed to read client certificate file: %w", err)
	}

	keyData, err := os.ReadFile(ap.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read client key file: %w", err)
	}

	certHash := sha256.Sum256(certPEMBlock)
	keyHash := sha256.Sum256(keyData)

	ap.mu.RLock()
	if ap.cachedCert != nil && ap.certFileHash == certHash && ap.keyFileHash == keyHash {
		cert := ap.cachedCert
		ap.mu.RUnlock()
		return cert, nil
	}
	ap.mu.RUnlock()

	var keyPEMBlock []byte
	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, errors.New("PEM data could not be found")
	}

	// nolint: staticcheck // We don't want to forbid users from using this encryption.
	if x509.IsEncryptedPEMBlock(block) {
		if ap.PrivateKeyPassphrase == "" {
			return nil, errors.New("client private key passphrase is needed, because the certificate is password encrypted")
		}
		// nolint: staticcheck // We don't want to forbid users from using this encryption.
		decryptedBlock, err := x509.DecryptPEMBlock(block, []byte(ap.PrivateKeyPassphrase))
		if err != nil {
			return nil, err
		}
		key, err := x509.ParsePKCS8PrivateKey(decryptedBlock)
		if err != nil {
			key, err = x509.ParsePKCS1PrivateKey(decryptedBlock)
			if err != nil {
				return nil, fmt.Errorf("private key should be a PEM or plain PKCS1 or PKCS8; parse error: %v", err)
			}
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is invalid")
		}
		keyPEMBlock = pem.EncodeToMemory(
			&pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
			},
		)
	} else {
		keyPEMBlock = keyData
	}

	cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public/private key pair: %v", err)
	}

	ap.mu.Lock()
	ap.cachedCert = &cert
	ap.certFileHash = certHash
	ap.keyFileHash = keyHash
	ap.lastLoadTime = time.Now()
	ap.mu.Unlock()

	return &cert, nil
}

func (ap *clientTLSAuthPlugin) NewClient(c Config) (*http.Client, error) {
	tlsConfig, err := DefaultTLSConfig(c)
	if err != nil {
		return nil, err
	}

	if ap.Cert == "" {
		return nil, errors.New("client certificate is needed when client TLS is enabled")
	}
	if ap.PrivateKey == "" {
		return nil, errors.New("private key is needed when client TLS is enabled")
	}

	tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return ap.loadCertificate()
	}

	var client *http.Client

	if c.TLS != nil && c.TLS.CACert != "" {
		client = DefaultRoundTripperClient(tlsConfig, *c.ResponseHeaderTimeoutSeconds)
	} else {
		if ap.CACert != "" {
			c.logger.Warn("Deprecated 'services[_].credentials.client_tls.ca_cert' configuration specified. Use 'services[_].tls.ca_cert' instead. See https://www.openpolicyagent.org/docs/latest/configuration/#services")
			caCert, err := os.ReadFile(ap.CACert)
			if err != nil {
				return nil, err
			}

			var caCertPool *x509.CertPool
			if ap.SystemCARequired {
				caCertPool, err = x509.SystemCertPool()
				if err != nil {
					return nil, err
				}
			} else {
				caCertPool = x509.NewCertPool()
			}

			ok := caCertPool.AppendCertsFromPEM(caCert)
			if !ok {
				return nil, errors.New("unable to parse and append CA certificate to certificate pool")
			}
			tlsConfig.RootCAs = caCertPool
		}

		client = DefaultRoundTripperClient(tlsConfig, *c.ResponseHeaderTimeoutSeconds)
	}

	return client, nil
}

func (*clientTLSAuthPlugin) Prepare(*http.Request) error {
	return nil
}
