// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"io"
	"os"
	"time"

	"github.com/open-policy-agent/opa/logging"
)

func (s *Server) getCertificate(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
	s.certMtx.RLock()
	defer s.certMtx.RUnlock()
	return s.cert, nil
}

func (s *Server) certLoop(logger logging.Logger) Loop {
	return func() error {
		for range time.NewTicker(s.certRefresh).C {
			certHash, err := hash(s.certFile)
			if err != nil {
				logger.Info("Failed to refresh server certificate: %s.", err.Error())
				continue
			}
			certKeyHash, err := hash(s.certKeyFile)
			if err != nil {
				logger.Info("Failed to refresh server certificate: %s.", err.Error())
				continue
			}

			s.certMtx.Lock()

			different := !bytes.Equal(s.certFileHash, certHash) ||
				!bytes.Equal(s.certKeyFileHash, certKeyHash)

			if different { // load and store
				newCert, err := tls.LoadX509KeyPair(s.certFile, s.certKeyFile)
				if err != nil {
					logger.Info("Failed to refresh server certificate: %s.", err.Error())
					s.certMtx.Unlock()
					continue
				}
				s.cert = &newCert
				s.certFileHash = certHash
				s.certKeyFileHash = certKeyHash
				logger.Debug("Refreshed server certificate.")
			}

			s.certMtx.Unlock()
		}

		return nil
	}
}

func hash(file string) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}
