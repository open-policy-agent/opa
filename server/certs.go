// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
	"crypto/tls"
	"errors"
	"time"

	"github.com/open-policy-agent/opa/logging"
)

func (s *Server) getCertificate(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cert := s.cert.Load()
	if cert == nil {
		return nil, errors.New("no certificate loaded")
	}
	return cert.(*tls.Certificate), nil
}

func (s *Server) certLoop(logger logging.Logger) Loop {
	return func() error {
		for range time.NewTicker(s.certRefresh).C {
			newCert, err := tls.LoadX509KeyPair(s.certFile, s.certKeyFile)
			if err != nil {
				logger.Info("Failed to refresh server certificate: %s.", err.Error())
				continue
			}
			logger.Debug("Refreshed server certificate.")
			s.cert.Store(&newCert)
		}

		return nil
	}
}
