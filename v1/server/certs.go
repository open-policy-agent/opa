// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/open-policy-agent/opa/internal/pathwatcher"
	"github.com/open-policy-agent/opa/v1/logging"
)

func (s *Server) getCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	s.tlsConfigMtx.RLock()
	defer s.tlsConfigMtx.RUnlock()
	return s.cert, nil
}

// reloadTLSConfig reloads the TLS config if the cert, key files or cert pool contents have changed.
func (s *Server) reloadTLSConfig(logger logging.Logger) error {
	s.tlsConfigMtx.Lock()
	defer s.tlsConfigMtx.Unlock()

	// reloading of the certificate key pair and the CA pool are independent operations,
	// though errors from either operation are aggregated.
	var errs error

	// if the server has a cert configured, then we need to check the cert and key for changes.
	if s.certFile != "" {
		newCert, certFileHash, certKeyFileHash, updated, err := reloadCertificateKeyPair(
			s.certFile,
			s.certKeyFile,
			s.certFileHash,
			s.certKeyFileHash,
			logger,
		)
		if err != nil {
			errs = errors.Join(errs, err)
		} else if updated {
			s.cert = newCert
			s.certFileHash = certFileHash
			s.certKeyFileHash = certKeyFileHash

			logger.Debug("Refreshed server certificate.")
		}
	}

	// if the server has a cert pool configured, also attempt to reload this
	if s.certPoolFile != "" {
		pool, certPoolFileHash, updated, err := reloadCertificatePool(s.certPoolFile, s.certPoolFileHash, logger)
		if err != nil {
			errs = errors.Join(errs, err)
		} else if updated {
			s.certPool = pool
			s.certPoolFileHash = certPoolFileHash
			logger.Debug("Refreshed server CA certificate pool.")
		}
	}

	return errs
}

// reloadCertificatePool loads the CA cert pool from the given file and returns a new pool if the file has changed.
func reloadCertificatePool(certPoolFile string, certPoolFileHash []byte, _ logging.Logger) (*x509.CertPool, []byte, bool, error) {
	certPoolHash, err := hash(certPoolFile)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to hash CA cert pool file: %w", err)
	}

	if bytes.Equal(certPoolFileHash, certPoolHash) {
		return nil, nil, false, nil
	}
	caCertPEM, err := os.ReadFile(certPoolFile)
	if err != nil {
		return nil, nil, false, fmt.Errorf("failed to read CA cert pool file %q: %w", certPoolFile, err)
	}

	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caCertPEM); !ok {
		return nil, nil, false, fmt.Errorf("failed to load CA cert pool file %q", certPoolFile)
	}

	return pool, certPoolHash, true, nil
}

// reloadCertificateKeyPair loads the certificate and key from the given files and returns a new certificate if either
// file has changed.
func reloadCertificateKeyPair(
	certFile, certKeyFile string,
	certFileHash, certKeyFileHash []byte,
	logger logging.Logger,
) (*tls.Certificate, []byte, []byte, bool, error) {
	certHash, err := hash(certFile)
	if err != nil {
		return nil, nil, nil, false, fmt.Errorf("failed to hash server certificate file: %w", err)
	}

	certKeyHash, err := hash(certKeyFile)
	if err != nil {
		return nil, nil, nil, false, fmt.Errorf("failed to hash server key file: %w", err)
	}

	differentCert := !bytes.Equal(certFileHash, certHash)
	differentKey := !bytes.Equal(certKeyFileHash, certKeyHash)

	if differentCert && !differentKey {
		logger.Warn("Server certificate file changed but server key file did not change.")
	}
	if !differentCert && differentKey {
		logger.Warn("Server key file changed but server certificate file did not change.")
	}

	if !differentCert && !differentKey {
		return nil, nil, nil, false, nil
	}

	newCert, err := tls.LoadX509KeyPair(certFile, certKeyFile)
	if err != nil {
		return nil, nil, nil, false, fmt.Errorf("server certificate key pair was not updated, update failed: %w", err)
	}

	return &newCert, certHash, certKeyHash, true, nil
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
			if (evt.Op & mask) == 0 {
				continue
			}

			// retry logic here handles cases where the files are still being written to as events are triggered.
			retries := 0
			for {
				err = s.reloadTLSConfig(s.manager.Logger())
				if err == nil {
					logger.Info("TLS config reloaded")
					break
				}

				retries++
				if retries >= 5 {
					logger.Error("Failed to reload TLS config after retrying: %s", err)
					break
				}

				time.Sleep(100 * time.Millisecond)
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
