// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rest

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
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

	// copy, we don't want to alter the default client's Transport
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ResponseHeaderTimeout = time.Duration(timeout) * time.Second
	tr.TLSClientConfig = t

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

type tokenEndpointResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

// oauth2ClientCredentialsAuthPlugin represents authentication via a bearer token in the HTTP Authorization header
// obtained through the OAuth2 client credentials flow
type oauth2ClientCredentialsAuthPlugin struct {
	TokenURL     string    `json:"token_url"`
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret"`
	Scopes       *[]string `json:"scopes,omitempty"`

	tokenCache    *oauth2Token
	tlsSkipVerify bool
}

type oauth2Token struct {
	Token     string
	ExpiresAt time.Time
}

func (ap *oauth2ClientCredentialsAuthPlugin) NewClient(c Config) (*http.Client, error) {
	t, err := defaultTLSConfig(c)
	if err != nil {
		return nil, err
	}

	// Inherit skip verify from the "parent" settings. Should this be configurable on the credentials too?
	ap.tlsSkipVerify = c.AllowInsureTLS

	if !strings.HasPrefix(ap.TokenURL, "https://") {
		return nil, errors.New("token_url required to use https scheme")
	}
	if ap.ClientID == "" || ap.ClientSecret == "" {
		return nil, errors.New("client_id and client_secret required")
	}
	if ap.Scopes == nil {
		ap.Scopes = &[]string{}
	}

	return defaultRoundTripperClient(t, *c.ResponseHeaderTimeoutSeconds), nil
}

// requestToken tries to obtain an access token using the client credentials flow
// https://tools.ietf.org/html/rfc6749#section-4.4
func (ap *oauth2ClientCredentialsAuthPlugin) requestToken() (*oauth2Token, error) {
	body := url.Values{"grant_type": []string{"client_credentials"}}
	if len(*ap.Scopes) > 0 {
		body["scope"] = []string{strings.Join(*ap.Scopes, " ")}
	}

	r, err := http.NewRequest("POST", ap.TokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetBasicAuth(ap.ClientID, ap.ClientSecret)

	client := defaultRoundTripperClient(&tls.Config{InsecureSkipVerify: ap.tlsSkipVerify}, 10)
	response, err := client.Do(r)
	if err != nil {
		return nil, err
	}

	bodyRaw, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("error in response from OAuth2 token endpoint: %v", string(bodyRaw))
	}

	var tokenResponse tokenEndpointResponse
	err = json.Unmarshal(bodyRaw, &tokenResponse)
	if err != nil {
		return nil, err
	}

	if strings.ToLower(tokenResponse.TokenType) != "bearer" {
		return nil, errors.New("unknown token type returned from token endpoint")
	}

	return &oauth2Token{
		Token:     strings.TrimSpace(tokenResponse.AccessToken),
		ExpiresAt: time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second),
	}, nil
}

func (ap *oauth2ClientCredentialsAuthPlugin) Prepare(req *http.Request) error {
	minTokenLifetime := float64(10)
	if ap.tokenCache == nil || ap.tokenCache.ExpiresAt.Sub(time.Now()).Seconds() < minTokenLifetime {
		logrus.Debugf("Requesting token from token_url %v", ap.TokenURL)
		token, err := ap.requestToken()
		if err != nil {
			return err
		}
		ap.tokenCache = token
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", ap.tokenCache.Token))
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
			return nil, errors.New("client certificate passphrase is needed, because the certificate is password encrypted")
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
