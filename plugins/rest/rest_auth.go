// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rest

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// defaultTLSConfig defines standard TLS configurations based on the Config
func defaultTLSConfig(c Config) (*tls.Config, error) {
	t := &tls.Config{}
	url, err := url.Parse(c.URL)
	if err != nil {
		return nil, err
	}
	if url.Scheme == "https" {
		t.InsecureSkipVerify = c.AllowInsureTLS
	}
	return t, nil
}

// defaultRoundTripperClient is a reasonable set of defaults for HTTP auth plugins
func defaultRoundTripperClient(t *tls.Config, timeout int64) *http.Client {
	// Ensure we use a http.Transport with proper settings: the zero values are not
	// a good choice, as they cause leaking connections:
	// https://github.com/golang/go/issues/19620
	//
	// Also, there's no simple way to copy the values from http.DefaultTransport,
	// see https://github.com/golang/go/issues/26013. Hence, we copy the settings
	// used in the golang sources,
	// https://github.com/golang/go/blob/5fae09b7386de26db59a1184f62fc7b22ec7667b/src/net/http/transport.go#L42-L53
	//   Copyright 2011 The Go Authors. All rights reserved.
	//   Use of this source code is governed by a BSD-style
	//   license that can be found in the LICENSE file.
	var tr http.RoundTripper = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: time.Duration(timeout) * time.Second,
		TLSClientConfig:       t,
	}

	// copy, we don't want to alter the default client's Transport
	c := *http.DefaultClient
	c.Transport = tr
	return &c
}

// defaultAuthPlugin represents baseline 'no auth' behavior if no alternative plugin is specified for a service
type defaultAuthPlugin struct{}

func (ap *defaultAuthPlugin) NewClient(c Config) (*http.Client, error) {
	t, err := defaultTLSConfig(c)
	if err != nil {
		return nil, err
	}
	return defaultRoundTripperClient(t, *c.ResponseHeaderTimeoutSeconds), nil
}

func (ap *defaultAuthPlugin) Prepare(req *http.Request) error {
	return nil
}

// bearerAuthPlugin represents authentication via a bearer token in the HTTP Authorization header
type bearerAuthPlugin struct {
	Token     string `json:"token"`
	TokenPath string `json:"token_path"`
	Scheme    string `json:"scheme,omitempty"`
}

func (ap *bearerAuthPlugin) NewClient(c Config) (*http.Client, error) {
	t, err := defaultTLSConfig(c)
	if err != nil {
		return nil, err
	}

	if ap.Token != "" && ap.TokenPath != "" {
		return nil, errors.New("invalid config: specify a value for either the \"token\" or \"token_path\" field")
	}

	if ap.Scheme == "" {
		ap.Scheme = "Bearer"
	}

	return defaultRoundTripperClient(t, *c.ResponseHeaderTimeoutSeconds), nil
}

func (ap *bearerAuthPlugin) Prepare(req *http.Request) error {
	token := ap.Token

	if ap.TokenPath != "" {
		bytes, err := ioutil.ReadFile(ap.TokenPath)
		if err != nil {
			return err
		}
		token = strings.TrimSpace(string(bytes))
	}

	req.Header.Add("Authorization", fmt.Sprintf("%v %v", ap.Scheme, token))
	return nil
}

// clientTLSAuthPlugin represents authentication via client certificate on a TLS connection
type clientTLSAuthPlugin struct {
	Cert                 string `json:"cert"`
	PrivateKey           string `json:"private_key"`
	PrivateKeyPassphrase string `json:"private_key_passphrase,omitempty"`
}

func (ap *clientTLSAuthPlugin) NewClient(c Config) (*http.Client, error) {
	tlsConfig, err := defaultTLSConfig(c)
	if err != nil {
		return nil, err
	}

	if ap.Cert == "" {
		return nil, errors.New("client certificate is needed when client TLS is enabled")
	}
	if ap.PrivateKey == "" {
		return nil, errors.New("private key is needed when client TLS is enabled")
	}

	var keyPEMBlock []byte
	data, err := ioutil.ReadFile(ap.PrivateKey)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("PEM data could not be found")
	}

	if x509.IsEncryptedPEMBlock(block) {
		if ap.PrivateKeyPassphrase == "" {
			return nil, errors.New("client certificate passphrase is need, because the certificate is password encrypted")
		}
		block, err := x509.DecryptPEMBlock(block, []byte(ap.PrivateKeyPassphrase))
		if err != nil {
			return nil, err
		}
		key, err := x509.ParsePKCS8PrivateKey(block)
		if err != nil {
			key, err = x509.ParsePKCS1PrivateKey(block)
			if err != nil {
				return nil, fmt.Errorf("private key should be a PEM or plain PKCS1 or PKCS8; parse error: %v", err)
			}
		}
		rsa, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is invalid")
		}
		keyPEMBlock = pem.EncodeToMemory(
			&pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: x509.MarshalPKCS1PrivateKey(rsa),
			},
		)
	} else {
		keyPEMBlock = data
	}

	certPEMBlock, err := ioutil.ReadFile(ap.Cert)
	if err != nil {
		return nil, err
	}

	cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return nil, err
	}

	tlsConfig.Certificates = []tls.Certificate{cert}
	client := defaultRoundTripperClient(tlsConfig, *c.ResponseHeaderTimeoutSeconds)
	return client, nil
}

func (ap *clientTLSAuthPlugin) Prepare(req *http.Request) error {
	return nil
}

// awsSigningAuthPlugin represents authentication using AWS V4 HMAC signing in the Authorization header
type awsSigningAuthPlugin struct {
	AWSEnvironmentCredentials *awsEnvironmentCredentialService `json:"environment_credentials,omitempty"`
	AWSMetadataCredentials    *awsMetadataCredentialService    `json:"metadata_credentials,omitempty"`
}

func (ap *awsSigningAuthPlugin) awsCredentialService() awsCredentialService {
	if ap.AWSEnvironmentCredentials != nil {
		return ap.AWSEnvironmentCredentials
	}
	return ap.AWSMetadataCredentials
}

func (ap *awsSigningAuthPlugin) NewClient(c Config) (*http.Client, error) {
	t, err := defaultTLSConfig(c)
	if err != nil {
		return nil, err
	}
	if (ap.AWSEnvironmentCredentials == nil && ap.AWSMetadataCredentials == nil) ||
		(ap.AWSEnvironmentCredentials != nil && ap.AWSMetadataCredentials != nil) {
		return nil, errors.New("exactly one AWS credential service must be specified when S3 signing is enabled")
	}
	if ap.AWSMetadataCredentials != nil {
		if ap.AWSMetadataCredentials.RegionName == "" {
			return nil, errors.New("at least aws_region must be specified for AWS metadata credential service")
		}
	}
	return defaultRoundTripperClient(t, *c.ResponseHeaderTimeoutSeconds), nil
}

func (ap *awsSigningAuthPlugin) Prepare(req *http.Request) error {
	logrus.Debug("Signing request with AWS credentials.")
	err := signV4(req, ap.awsCredentialService(), time.Now())
	return err
}
