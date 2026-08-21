// Copyright 2017 johandorland ( https://github.com/johandorland )
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gojsonschema

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

type jsonSchemaTest struct {
	Description string `json:"description"`
	// Some tests may not always pass, so some tests are manually edited to include
	// an extra attribute whether that specific test should be disabled and skipped
	Disabled bool                 `json:"disabled"`
	Schema   any                  `json:"schema"`
	Tests    []jsonSchemaTestCase `json:"tests"`
}
type jsonSchemaTestCase struct {
	Description string `json:"description"`
	Data        any    `json:"data"`
	Valid       bool   `json:"valid"`
}

// Skip any directories not named appropiately
// filepath.Walk will also visit files in the root of the test directory
var testDirectories = regexp.MustCompile(`(draft\d+)`)
var draftMapping = map[string]Draft{
	"draft4": Draft4,
	"draft6": Draft6,
	"draft7": Draft7,
}

func executeTests(t *testing.T, path string) error {
	file, err := os.Open(path)
	if err != nil {
		t.Errorf("Error (%s)\n", err.Error())
	}
	fmt.Println(file.Name())

	var tests []jsonSchemaTest
	d := json.NewDecoder(file)
	d.UseNumber()
	err = d.Decode(&tests)

	if err != nil {
		t.Errorf("Error (%s)\n", err.Error())
	}

	draft := Hybrid
	if m := testDirectories.FindString(path); m != "" {
		draft = draftMapping[m]
	}

	for _, test := range tests {
		fmt.Println("    " + test.Description)

		if test.Disabled {
			continue
		}

		testSchemaLoader := NewRawLoader(test.Schema)
		sl := NewSchemaLoader()
		sl.Draft = draft
		sl.Validate = true
		testSchema, err := sl.Compile(testSchemaLoader)

		if err != nil {
			t.Errorf("Error (%s)\n", err.Error())
		}

		for _, testCase := range test.Tests {
			testDataLoader := NewRawLoader(testCase.Data)
			result, err := testSchema.Validate(testDataLoader)

			if err != nil {
				t.Errorf("Error (%s)\n", err.Error())
			}

			if result.Valid() != testCase.Valid {
				schemaString, _ := marshalToJSONString(test.Schema)
				testCaseString, _ := marshalToJSONString(testCase.Data)

				t.Errorf("Test failed : %s\n"+
					"%s.\n"+
					"%s.\n"+
					"expects: %t, given %t\n"+
					"Schema: %s\n"+
					"Data: %s\n",
					file.Name(),
					test.Description,
					testCase.Description,
					testCase.Valid,
					result.Valid(),
					*schemaString,
					*testCaseString)
			}
		}
	}
	return nil
}

func TestSuite(t *testing.T) {

	wd, err := os.Getwd()
	if err != nil {
		panic(err.Error())
	}
	wd = filepath.Join(wd, "testdata")

	go func() {
		err := http.ListenAndServe("localhost:1234", http.FileServer(http.Dir(filepath.Join(wd, "remotes"))))
		if err != nil {

			panic(err.Error())
		}
	}()

	err = filepath.Walk(wd, func(path string, fileInfo os.FileInfo, _ error) error {
		if fileInfo.IsDir() && path != wd && !testDirectories.MatchString(fileInfo.Name()) {
			return filepath.SkipDir
		}
		if !strings.HasSuffix(fileInfo.Name(), ".json") {
			return nil
		}
		return executeTests(t, path)
	})
	if err != nil {
		t.Errorf("Error (%s)\n", err.Error())
	}
}

func TestFormats(t *testing.T) {
	// NOTE(sr): Go 1.26 tightened url.Parse to reject ambiguous (unbracketed)
	// colons in the host of http(s) URLs by default (see the urlstrictcolons
	// GODEBUG setting, https://go.dev/doc/go1.26#net-url). One of the upstream
	// JSON-Schema-Test-Suite cases for the (lenient) "iri" format relies on the
	// old, lenient parsing of a bracketless IPv6 host, so restore it here.
	t.Setenv("GODEBUG", "urlstrictcolons=0")

	wd, err := os.Getwd()
	if err != nil {
		panic(err.Error())
	}
	wd = filepath.Join(wd, "testdata")

	dirs, err := os.ReadDir(wd)

	if err != nil {
		panic(err.Error())
	}

	for _, dir := range dirs {
		if testDirectories.MatchString(dir.Name()) {
			formatJSONFile := filepath.Join(wd, dir.Name(), "optional", "format.json")
			if _, err = os.Stat(formatJSONFile); err == nil {
				err = executeTests(t, formatJSONFile)
			} else {
				err = nil
			}

			if err != nil {
				t.Errorf("Error (%s)\n", err.Error())
			}

			formatsDirectory := filepath.Join(wd, dir.Name(), "optional", "format")
			err = filepath.Walk(formatsDirectory, func(path string, fileInfo os.FileInfo, _ error) error {
				if fileInfo == nil || !strings.HasSuffix(fileInfo.Name(), ".json") {
					return nil
				}
				return executeTests(t, path)
			})

			if err != nil {
				t.Errorf("Error (%s)\n", err.Error())
			}
		}
	}
}

func TestAllowNetIsPerSchemaLoader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"type": "string"}`)
	}))
	defer srv.Close()

	srvURL, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf(`{"$ref": %q}`, srv.URL+"/schema.json")

	compile := func(allowNet []string) error {
		sl := NewSchemaLoader()
		sl.AllowNet = allowNet
		_, err := sl.Compile(NewStringLoader(schema))
		return err
	}

	tests := []struct {
		note       string
		allowNet   []string
		wantDenied bool
	}{
		{note: "nil list permits any host", allowNet: nil},
		{note: "empty list permits no host", allowNet: []string{}, wantDenied: true},
		{note: "listed host is permitted", allowNet: []string{srvURL.Hostname()}},
		{note: "unlisted host is denied", allowNet: []string{"example.com"}, wantDenied: true},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			err := compile(tc.allowNet)
			if tc.wantDenied {
				if err == nil {
					t.Fatal("expected remote reference to be denied, but compilation succeeded")
				}
				if !strings.Contains(err.Error(), "remote reference loading disabled") {
					t.Fatalf("expected remote reference loading to be disabled, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected remote reference to be permitted, got %v", err)
			}
		})
	}

	// Loaders with conflicting allowlists, used concurrently, must each honour
	// their own -- the permissive one must not open up the restrictive one.
	t.Run("conflicting allowlists used concurrently", func(t *testing.T) {
		var wg sync.WaitGroup
		for range 20 {
			wg.Add(2)
			go func() {
				defer wg.Done()
				if err := compile(nil); err != nil {
					t.Errorf("permissive loader: expected success, got %v", err)
				}
			}()
			go func() {
				defer wg.Done()
				if err := compile([]string{}); err == nil {
					t.Error("restrictive loader: expected remote reference to be denied")
				}
			}()
		}
		wg.Wait()
	})
}

func TestAllowNetIsCheckedOnRedirects(t *testing.T) {
	var srv *httptest.Server

	mux := http.NewServeMux()
	mux.HandleFunc("/schema.json", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"type": "string"}`)
	})
	mux.HandleFunc("/leave", func(w http.ResponseWriter, r *http.Request) {
		// The same address under a different host name, which is what the
		// allowlist matches on.
		viaLocalhost := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)
		http.Redirect(w, r, viaLocalhost+"/schema.json", http.StatusFound)
	})
	mux.HandleFunc("/hop-then-leave", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+"/leave", http.StatusFound)
	})
	mux.HandleFunc("/hop-then-stay", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+"/schema.json", http.StatusFound)
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	srvURL, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		note       string
		entry      string
		wantDenied bool
	}{
		{note: "first hop leaves the allowlist", entry: "/leave", wantDenied: true},
		{note: "later hop leaves the allowlist", entry: "/hop-then-leave", wantDenied: true},
		{note: "chain stays within the allowlist", entry: "/hop-then-stay"},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			sl := NewSchemaLoader()
			sl.AllowNet = []string{srvURL.Hostname()} // permits 127.0.0.1, not localhost
			_, err := sl.Compile(NewStringLoader(fmt.Sprintf(`{"$ref": %q}`, srv.URL+tc.entry)))

			if tc.wantDenied {
				if err == nil {
					t.Fatal("expected the redirect to a non-allowlisted host to be denied")
				}
				if !strings.Contains(err.Error(), "remote reference loading disabled") {
					t.Fatalf("expected remote reference loading to be disabled, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected the redirect chain to be permitted, got %v", err)
			}
		})
	}
}

// Setting CheckRedirect opts out of net/http's own hop limit, so the limit has
// to be enforced by the loader instead. Nothing else bounds the exchange -- the
// client applies no timeout -- so without the limit a host that redirects to
// itself keeps a fetch going indefinitely.
func TestRemoteRefRedirectsAreBounded(t *testing.T) {
	var hops int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops++
		http.Redirect(w, r, "/again", http.StatusFound)
	}))
	defer srv.Close()

	// The hop limit fires in milliseconds against a local server; the deadline
	// is only here so a regression fails the test instead of hanging it.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sl := NewSchemaLoader()
	sl.Context = ctx

	_, err := sl.Compile(NewStringLoader(fmt.Sprintf(`{"$ref": %q}`, srv.URL+"/schema.json")))
	if err == nil {
		t.Fatal("expected the redirect loop to be stopped")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("stopped after %d redirects", maxRemoteRefRedirects)) {
		t.Fatalf("expected the redirect loop to be stopped after %d redirects, got %v", maxRemoteRefRedirects, err)
	}
	if hops > maxRemoteRefRedirects+1 {
		t.Fatalf("expected at most %d requests, got %d", maxRemoteRefRedirects+1, hops)
	}
}

// A cancelled caller -- an aborted or timed-out query, say -- should not leave
// a remote reference fetch running behind it.
func TestRemoteRefFetchHonoursContext(t *testing.T) {
	release := make(chan struct{})
	defer close(release)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
		fmt.Fprint(w, `{"type": "string"}`)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sl := NewSchemaLoader()
	sl.Context = ctx

	done := make(chan error, 1)
	go func() {
		_, err := sl.Compile(NewStringLoader(fmt.Sprintf(`{"$ref": %q}`, srv.URL+"/schema.json")))
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected the cancelled fetch to fail")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected a context cancellation error, got %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("fetch did not observe the cancelled context")
	}
}

// The loaders reached by the compile-time type-checking path are built without
// a context; they must still work rather than panicking on a nil one.
func TestRemoteRefFetchWithoutContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"type": "string"}`)
	}))
	defer srv.Close()

	sl := NewSchemaLoader() // Context left unset
	if _, err := sl.Compile(NewStringLoader(fmt.Sprintf(`{"$ref": %q}`, srv.URL+"/schema.json"))); err != nil {
		t.Fatalf("expected the fetch to succeed, got %v", err)
	}
}
