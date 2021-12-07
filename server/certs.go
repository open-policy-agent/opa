// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import "crypto/tls"

func (s *Server) getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return s.cert, nil
}
