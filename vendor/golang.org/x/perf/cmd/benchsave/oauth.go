// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var oauthConfig = oauth2.Config{
	ClientID:     "337869179296-kfjqcm11uodlrj39ifek1adtjjfb0b1p.apps.googleusercontent.com",
	ClientSecret: "zOMue3fEEUnz4Em39Ia_-4TN",
	Endpoint:     google.Endpoint,
	Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
	RedirectURL:  "oob",
}

// newTokenSource constructs a token source that will try to read a
// cached token from a file and if missing or invalid will prompt the
// user to refresh it.
func newTokenSource() *tokenReplacer {
	// nil token will cause a refresh
	tok, _ := readToken()
	return &tokenReplacer{tok, oauthConfig.TokenSource(context.Background(), tok), &tokenPrompt{}}
}

// tokenPrompt is a TokenSource that interactively prompts the user
// for a token using copy-paste.
type tokenPrompt struct{}

// Token obtains a new token from the user.
func (*tokenPrompt) Token() (*oauth2.Token, error) {
	url := oauthConfig.AuthCodeURL("")

	fmt.Fprintf(os.Stderr, "benchsave must authenticate to %s.\n"+
		"Note that uploaded files will be publicly recorded with your email address.\n"+
		"\n"+
		"%s\n"+
		"\n"+
		"Visit the URL above and enter the auth code: ", *server, url)

	var code string
	fmt.Scanln(&code)

	ctx := context.Background()
	return oauthConfig.Exchange(ctx, code)
}

// tokenReplacer is a TokenSource that tries to obtain a token from ts, and if it fails, obtains a token from tsr and uses it to replace ts. New tokens are cached on disk.
type tokenReplacer struct {
	lastTok *oauth2.Token
	ts, tsr oauth2.TokenSource
}

// Token returns an existing or refreshed token, or uses t.tsr to obtain a new token.
func (t *tokenReplacer) Token() (tok *oauth2.Token, err error) {
	defer func() {
		// If the AccessToken field changes, cache the new token.
		if tok != nil && (t.lastTok == nil || tok.AccessToken != t.lastTok.AccessToken) {
			if err := writeToken(tok); err != nil {
				log.Printf("cannot cache authentication: %v", err)
			}
		}
	}()
	tok, err = t.ts.Token()
	if err == nil {
		return
	}
	tok, err = t.tsr.Token()
	if err == nil {
		t.ts = oauthConfig.TokenSource(context.Background(), tok)
	}
	return
}

// tokenFilePath returns "$XDG_CONFIG_HOME/.config/benchsave/token.json"
// with recursive expansion of defaults.
func tokenFilePath() string {
	d := os.Getenv("XDG_CONFIG_HOME")
	if d == "" {
		home := os.Getenv("HOME")
		if home == "" {
			u, err := user.Current()
			if err != nil {
				log.Fatal(err)
			}
			home = u.HomeDir
		}
		d = filepath.Join(home, ".config")
	}
	return filepath.Join(d, "benchsave", "token.json")
}

// readToken obtains a token from the filesystem.
// If there is no valid token found, it returns a nil token and a reason.
func readToken() (*oauth2.Token, error) {
	data, err := ioutil.ReadFile(tokenFilePath())
	if err != nil {
		return nil, err
	}
	tok := &oauth2.Token{}
	if err := json.Unmarshal(data, tok); err != nil {
		return nil, err
	}
	return tok, nil
}

// writeToken writes the token to the filesystem.
func writeToken(tok *oauth2.Token) (err error) {
	p := tokenFilePath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(tok)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(p, data, 0600)
}
