// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/open-policy-agent/opa/internal/pathwatcher"
	"github.com/open-policy-agent/opa/logging"
)

func (s *Server) getCertificate(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
	s.tlsConfigMtx.RLock()
	defer s.tlsConfigMtx.RUnlock()
	return s.cert, nil
}

func (s *Server) reloadTLSConfig(logger logging.Logger) error {
	s.tlsConfigMtx.Lock()
	defer s.tlsConfigMtx.Unlock()

	// if the server has a cert configured, then we need to check the cert and key for changes.
	if s.certFile != "" {
		certHash, err := hash(s.certFile)
		if err != nil {
			return fmt.Errorf("failed to check server certificate: %w", err)
		}

		certKeyHash, err := hash(s.certKeyFile)
		if err != nil {
			return fmt.Errorf("failed to check server key: %w", err)
		}

		different := !bytes.Equal(s.certFileHash, certHash) ||
			!bytes.Equal(s.certKeyFileHash, certKeyHash)

		if different { // load and store
			newCert, err := tls.LoadX509KeyPair(s.certFile, s.certKeyFile)
			if err != nil {
				return fmt.Errorf("failed to refresh server certificate: %w", err)
			}
			s.cert = &newCert
			s.certFileHash = certHash
			s.certKeyFileHash = certKeyHash
			logger.Debug("Refreshed server certificate.")
		}
	}

	// do not attempt to reload the ca cert pool if it has not been configured.
	if s.certPoolFile != "" {
		certPoolHash, err := hash(s.certPoolFile)
		if err != nil {
			return fmt.Errorf("failed to refresh CA cert pool: %w", err)
		}

		if !bytes.Equal(s.certPoolFileHash, certPoolHash) {
			caCertPEM, err := os.ReadFile(s.certPoolFile)
			if err != nil {
				return fmt.Errorf("failed to read CA cert pool file: %w", err)
			}

			pool := x509.NewCertPool()
			if ok := pool.AppendCertsFromPEM(caCertPEM); !ok {
				return fmt.Errorf("failed to parse CA cert pool file %q", s.certPoolFile)
			}

			s.certPool = pool
		}
	}

	return nil
}

func (s *Server) certLoopPolling(logger logging.Logger) Loop {
	return func() error {
		for range time.NewTicker(s.certRefresh).C {
			err := s.reloadTLSConfig(logger)
			if err != nil {
				logger.Error(fmt.Sprintf("Failed to reload TLS config: %s", err))
			}
		}

		return nil
	}
}

func (s *Server) certLoopNotify(logger logging.Logger) Loop {
	return func() error {

		var paths []string

		// if a cert file is set, then we want to watch the cert and key
		if s.certFile != "" {
			paths = append(paths, s.certFile, s.certKeyFile)
		}

		// if a cert pool file is set, then we want to watch the cert pool. This might be set without the cert and key
		// being set too.
		if s.certPoolFile != "" {
			paths = append(paths, s.certPoolFile)
		}

		watcher, err := pathwatcher.CreatePathWatcher(paths)
		if err != nil {
			return fmt.Errorf("failed to create tls path watcher: %w", err)
		}

		for evt := range watcher.Events {
			removalMask := fsnotify.Remove | fsnotify.Rename
			mask := fsnotify.Create | fsnotify.Write | removalMask
			if (evt.Op & mask) != 0 {
				err = s.reloadTLSConfig(s.manager.Logger())
				if err != nil {
					logger.Error("failed to reload TLS config: %s", err)
				}
				logger.Info("TLS config reloaded")
			}
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
