// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/util"
)

func builtinCryptoX509ParseCertificates(a ast.Value) (ast.Value, error) {

	str, err := builtinBase64Decode(a)
	if err != nil {
		return nil, err
	}

	certs, err := x509.ParseCertificates([]byte(str.(ast.String)))
	if err != nil {
		return nil, err
	}

	bs, err := json.Marshal(certs)
	if err != nil {
		return nil, err
	}

	var x interface{}

	if err := util.UnmarshalJSON(bs, &x); err != nil {
		return nil, err
	}

	return ast.InterfaceToValue(x)
}

func init() {
	RegisterFunctionalBuiltin1(ast.CryptoX509ParseCertificates.Name, builtinCryptoX509ParseCertificates)
}

// createRootCAs creates a new Cert Pool from scratch or adds to a copy of System Certs
func createRootCAs(tlsCACertFile string, tlsCACertEnvVar []byte, tlsUseSystemCerts bool) (*x509.CertPool, error) {

	var newRootCAs *x509.CertPool

	if tlsUseSystemCerts {
		systemCertPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}
		newRootCAs = systemCertPool
	} else {
		newRootCAs = x509.NewCertPool()
	}

	if len(tlsCACertFile) > 0 {
		// Append our cert to the system pool
		caCert, err := readCertFromFile(tlsCACertFile)
		if err != nil {
			return nil, err
		}
		if ok := newRootCAs.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("could not append CA cert from %q", tlsCACertFile)
		}
	}

	if len(tlsCACertEnvVar) > 0 {
		// Append our cert to the system pool
		if ok := newRootCAs.AppendCertsFromPEM(tlsCACertEnvVar); !ok {
			return nil, fmt.Errorf("error appending cert from env var %q into system certs", tlsCACertEnvVar)
		}
	}

	return newRootCAs, nil
}

// ReadCertFromFile reads a cert from file
func readCertFromFile(localCertFile string) ([]byte, error) {
	// Read in the cert file
	certPEM, err := ioutil.ReadFile(localCertFile)
	if err != nil {
		return nil, err
	}
	return certPEM, nil
}

// ReadKeyFromFile reads a key from file
func readKeyFromFile(localKeyFile string) ([]byte, error) {
	// Read in the cert file
	key, err := ioutil.ReadFile(localKeyFile)
	if err != nil {
		return nil, err
	}
	return key, nil
}
