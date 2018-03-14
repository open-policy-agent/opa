// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package rest implements a REST client for communicating with remote services.
package rest

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/open-policy-agent/opa/util"
	"github.com/sirupsen/logrus"
)

// Config represents configuration for a REST client.
type Config struct {
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers"`
	Credentials struct {
		Bearer *struct {
			Scheme string `json:"scheme,omitempty"`
			Token  string `json:"token"`
		} `json:"bearer,omitempty"`
	} `json:"credentials"`
}

func (c *Config) validateAndInjectDefaults() error {
	c.URL = strings.TrimRight(c.URL, "/")
	_, err := url.Parse(c.URL)
	if c.Credentials.Bearer != nil {
		if c.Credentials.Bearer.Scheme == "" {
			c.Credentials.Bearer.Scheme = "Bearer"
		}
	}
	return err
}

// Client implements an HTTP/REST client for communicating with remote
// services.
type Client struct {
	Client  http.Client
	config  Config
	headers map[string]string
}

// New returns a new Client for config.
func New(config []byte) (Client, error) {
	var parsedConfig Config

	if err := util.Unmarshal(config, &parsedConfig); err != nil {
		return Client{}, err
	}

	return Client{config: parsedConfig}, parsedConfig.validateAndInjectDefaults()
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

// Do executes a request using the client.
func (c Client) Do(ctx context.Context, method, path string) (*http.Response, error) {

	path = strings.Trim(path, "/")

	url := c.config.URL + "/" + path
	req, err := http.NewRequest(method, url, nil)
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

	return c.Client.Do(req)
}
