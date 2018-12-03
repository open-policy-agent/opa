// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package rest implements a REST client for communicating with remote services.
package rest

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/open-policy-agent/opa/util"
	"github.com/sirupsen/logrus"
)

// Config represents configuration for a REST client.
type Config struct {
	Name           string            `json:"name"`
	URL            string            `json:"url"`
	Headers        map[string]string `json:"headers"`
	AllowInsureTLS bool              `json:"allow_insecure_tls,omitempty"`
	Credentials    struct {
		Bearer *struct {
			Scheme string `json:"scheme,omitempty"`
			Token  string `json:"token"`
		} `json:"bearer,omitempty"`
		ClientTLS *struct {
			Cert                 string `json:"cert"`
			PrivateKey           string `json:"private_key"`
			PrivateKeyPassphrase string `json:"private_key_passphrase,omitempty"`
		} `json:"client_tls,omitempty"`
	} `json:"credentials"`
}

func (c *Config) validateAndInjectDefaults() (*tls.Config, error) {
	c.URL = strings.TrimRight(c.URL, "/")
	url, err := url.Parse(c.URL)
	if err != nil {
		return nil, err
	}
	if c.Credentials.Bearer != nil {
		if c.Credentials.Bearer.Scheme == "" {
			c.Credentials.Bearer.Scheme = "Bearer"
		}
	}
	tlsConfig := &tls.Config{}
	if url.Scheme == "https" {
		tlsConfig.InsecureSkipVerify = c.AllowInsureTLS
	}
	if c.Credentials.ClientTLS != nil {
		if err := c.readCertificate(tlsConfig); err != nil {
			return nil, err
		}
	}
	return tlsConfig, err
}

func (c *Config) readCertificate(t *tls.Config) error {
	if c.Credentials.ClientTLS.Cert == "" {
		return errors.New("client certificate is needed when client TLS is enabled")
	}
	if c.Credentials.ClientTLS.PrivateKey == "" {
		return errors.New("private key is needed when client TLS is enabled")
	}

	var keyPEMBlock []byte
	data, err := ioutil.ReadFile(c.Credentials.ClientTLS.PrivateKey)
	if err != nil {
		return err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return errors.New("PEM data could not be found")
	}

	if x509.IsEncryptedPEMBlock(block) {
		if c.Credentials.ClientTLS.PrivateKeyPassphrase == "" {
			return errors.New("client certificate passphrase is need, because the certificate is password encrypted")
		}
		block, err := x509.DecryptPEMBlock(block, []byte(c.Credentials.ClientTLS.PrivateKeyPassphrase))
		if err != nil {
			return err
		}
		key, err := x509.ParsePKCS8PrivateKey(block)
		if err != nil {
			key, err = x509.ParsePKCS1PrivateKey(block)
			if err != nil {
				return fmt.Errorf("private key should be a PEM or plain PKCS1 or PKCS8; parse error: %v", err)
			}
		}
		rsa, ok := key.(*rsa.PrivateKey)
		if !ok {
			return errors.New("private key is invalid")
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

	certPEMBlock, err := ioutil.ReadFile(c.Credentials.ClientTLS.Cert)
	if err != nil {
		return err
	}

	cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return err
	}

	t.Certificates = []tls.Certificate{cert}
	return nil
}

// Client implements an HTTP/REST client for communicating with remote
// services.
type Client struct {
	Client  http.Client
	bytes   *[]byte
	json    *interface{}
	config  Config
	headers map[string]string
}

// Name returns an option that overrides the service name on the client.
func Name(s string) func(*Client) {
	return func(c *Client) {
		c.config.Name = s
	}
}

// New returns a new Client for config.
func New(config []byte, opts ...func(*Client)) (Client, error) {
	var parsedConfig Config

	if err := util.Unmarshal(config, &parsedConfig); err != nil {
		return Client{}, err
	}

	tlsConfig, err := parsedConfig.validateAndInjectDefaults()
	if err != nil {
		return Client{}, err
	}

	client := Client{
		config: parsedConfig,
		Client: http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
		},
	}

	for _, f := range opts {
		f(&client)
	}

	return client, nil
}

// Service returns the name of the service this Client is configured for.
func (c Client) Service() string {
	return c.config.Name
}

// WithHeader returns a shallow copy of the client with a header to include the
// requests.
func (c Client) WithHeader(k, v string) Client {
	if v == "" {
		return c
	}
	if c.headers == nil {
		c.headers = map[string]string{}
	}
	c.headers[k] = v
	return c
}

// WithJSON returns a shallow copy of the client with the JSON value set as the
// message body to include the requests. This function sets the Content-Type
// header.
func (c Client) WithJSON(body interface{}) Client {
	c = c.WithHeader("Content-Type", "application/json")
	c.json = &body
	return c
}

// WithBytes returns a shallow copy of the client with the bytes set as the
// message body to include in the requests.
func (c Client) WithBytes(body []byte) Client {
	c.bytes = &body
	return c
}

// Do executes a request using the client.
func (c Client) Do(ctx context.Context, method, path string) (*http.Response, error) {

	path = strings.Trim(path, "/")

	var body io.Reader

	if c.bytes != nil {
		buf := bytes.NewBuffer(*c.bytes)
		body = buf
	} else if c.json != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(*c.json); err != nil {
			return nil, err
		}
		body = &buf
	}

	url := c.config.URL + "/" + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	headers := map[string]string{}

	// Set authorization header for credentials.
	if c.config.Credentials.Bearer != nil {
		req.Header.Add("Authorization", fmt.Sprintf("%v %v", c.config.Credentials.Bearer.Scheme, c.config.Credentials.Bearer.Token))
	}

	// Copy custom headers from config.
	for key, value := range c.config.Headers {
		headers[key] = value
	}

	// Overwrite with headers set directly on client.
	for key, value := range c.headers {
		headers[key] = value
	}

	for key, value := range headers {
		req.Header.Add(key, value)
	}

	req = req.WithContext(ctx)

	logrus.WithFields(logrus.Fields{
		"method":  method,
		"url":     url,
		"headers": req.Header,
	}).Debug("Sending request.")

	resp, err := c.Client.Do(req)
	if resp != nil {
		// Only log for debug purposes. If an error occurred, the caller should handle
		// that. In the non-error case, the caller may not do anything.
		logrus.WithFields(logrus.Fields{
			"method":  method,
			"url":     url,
			"status":  resp.Status,
			"headers": resp.Header,
		}).Debug("Received response.")
	}

	return resp, err
}
