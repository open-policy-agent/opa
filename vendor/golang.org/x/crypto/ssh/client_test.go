// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssh

import (
	"net"
	"strings"
	"testing"
)

func testClientVersion(t *testing.T, config *ClientConfig, expected string) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	receivedVersion := make(chan string, 1)
	config.HostKeyCallback = InsecureIgnoreHostKey()
	go func() {
		version, err := readVersion(serverConn)
		if err != nil {
			receivedVersion <- ""
		} else {
			receivedVersion <- string(version)
		}
		serverConn.Close()
	}()
	NewClientConn(clientConn, "", config)
	actual := <-receivedVersion
	if actual != expected {
		t.Fatalf("got %s; want %s", actual, expected)
	}
}

func TestCustomClientVersion(t *testing.T) {
	version := "Test-Client-Version-0.0"
	testClientVersion(t, &ClientConfig{ClientVersion: version}, version)
}

func TestDefaultClientVersion(t *testing.T) {
	testClientVersion(t, &ClientConfig{}, packageVersion)
}

func TestHostKeyCheck(t *testing.T) {
	for _, tt := range []struct {
		name      string
		wantError string
		key       PublicKey
	}{
		{"no callback", "must specify HostKeyCallback", nil},
		{"correct key", "", testSigners["rsa"].PublicKey()},
		{"mismatch", "mismatch", testSigners["ecdsa"].PublicKey()},
	} {
		c1, c2, err := netPipe()
		if err != nil {
			t.Fatalf("netPipe: %v", err)
		}
		defer c1.Close()
		defer c2.Close()
		serverConf := &ServerConfig{
			NoClientAuth: true,
		}
		serverConf.AddHostKey(testSigners["rsa"])

		go NewServerConn(c1, serverConf)
		clientConf := ClientConfig{
			User: "user",
		}
		if tt.key != nil {
			clientConf.HostKeyCallback = FixedHostKey(tt.key)
		}

		_, _, _, err = NewClientConn(c2, "", &clientConf)
		if err != nil {
			if tt.wantError == "" || !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("%s: got error %q, missing %q", tt.name, err.Error(), tt.wantError)
			}
		} else if tt.wantError != "" {
			t.Errorf("%s: succeeded, but want error string %q", tt.name, tt.wantError)
		}
	}
}

func TestBannerCallback(t *testing.T) {
	c1, c2, err := netPipe()
	if err != nil {
		t.Fatalf("netPipe: %v", err)
	}
	defer c1.Close()
	defer c2.Close()

	serverConf := &ServerConfig{
		PasswordCallback: func(conn ConnMetadata, password []byte) (*Permissions, error) {
			return &Permissions{}, nil
		},
		BannerCallback: func(conn ConnMetadata) string {
			return "Hello World"
		},
	}
	serverConf.AddHostKey(testSigners["rsa"])
	go NewServerConn(c1, serverConf)

	var receivedBanner string
	var bannerCount int
	clientConf := ClientConfig{
		Auth: []AuthMethod{
			Password("123"),
		},
		User:            "user",
		HostKeyCallback: InsecureIgnoreHostKey(),
		BannerCallback: func(message string) error {
			bannerCount++
			receivedBanner = message
			return nil
		},
	}

	_, _, _, err = NewClientConn(c2, "", &clientConf)
	if err != nil {
		t.Fatal(err)
	}

	if bannerCount != 1 {
		t.Errorf("got %d banners; want 1", bannerCount)
	}

	expected := "Hello World"
	if receivedBanner != expected {
		t.Fatalf("got %s; want %s", receivedBanner, expected)
	}
}
