package http_test

import (
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/test/e2e"
)

var testRuntime *e2e.TestRuntime

func TestMain(m *testing.M) {
	flag.Parse()

	var err error
	testRuntime, err = e2e.NewTestRuntime(e2e.NewAPIServerTestParams())
	if err != nil {
		os.Exit(1)
	}

	os.Exit(testRuntime.RunTests(m))
}

func TestHttpSendInterQueryForceCache(t *testing.T) {
	tests := []struct {
		note        string
		respHeaders map[string][]string
		noForce     bool
	}{
		{
			note:        "http.send GET no force_cache",
			respHeaders: map[string][]string{},
			noForce:     true,
		},
		{
			note:        "http.send GET force_cache default headers",
			respHeaders: map[string][]string{},
		},
		{
			note:        "http.send GET force_cache no Date header",
			respHeaders: map[string][]string{"Date": nil},
		},
		{
			note:        "http.send GET force_cache Date ignored",
			respHeaders: map[string][]string{"Date": {"Wed, 31 Dec 2005 07:28:00 GMT"}},
		},
		{
			note:        "http.send GET force_cache Expires ignored",
			respHeaders: map[string][]string{"Expires": {"Wed, 31 Dec 2005 07:28:00 GMT"}},
		},
		{
			note:        "http.send GET force_cache Cache-Control no-cache ignored",
			respHeaders: map[string][]string{"Cache-Control": {"no-store"}},
		},
		{
			note:        "http.send GET force_cache Cache-Control max-age ignored",
			respHeaders: map[string][]string{"Cache-Control": {"no-store", "max-age=0"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			counter := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				counter++
				w.Header()["Content-Type"] = []string{"application/json"}
				for k, v := range tc.respHeaders {
					w.Header()[k] = v
				}
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(fmt.Sprintf(`{"c": %d}`, counter)))
				if err != nil {
					t.Fatal(err)
				}
			}))
			defer ts.Close()

			var module string
			if tc.noForce {
				module = `
				package test
			
				response := http.send({
					"method": "get",
					"url": "%s"
				}).body
				`
			} else {
				module = `
				package test
			
				response := http.send({
					"method": "get",
					"url": "%s",
					"force_cache": true,
					"force_cache_duration_seconds": 60
				}).body
				`
			}

			// Since we are using the same request data for all tests (and only the response headers differ), we need
			// to ensure that each test has some distinct identifier appended to the URL, or else the same cache key
			// would be used for all of the tests!
			url := ts.URL + "?test=" + fmt.Sprintf("%x", md5.Sum([]byte(tc.note)))

			if err := testRuntime.UploadPolicy("test", strings.NewReader(fmt.Sprintf(module, url))); err != nil {
				t.Fatal(err)
			}

			parsedBody := struct {
				Result map[string]int `json:"result"`
			}{}
			expect := map[string]int{"c": 1}

			resultJSON, err := testRuntime.GetDataWithInput("test/response", map[string]string{})
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(resultJSON, &parsedBody); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(parsedBody.Result, expect) {
				t.Errorf("Expected response %v, got %v", expect, parsedBody.Result)
			}

			// Repeat once more to see if the result is cached between queries

			if tc.noForce {
				expect = map[string]int{"c": 2}
			}

			resultJSON, err = testRuntime.GetDataWithInput("test/response", map[string]string{})
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(resultJSON, &parsedBody); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(parsedBody.Result, expect) {
				t.Errorf("Expected response %v, got %v", expect, parsedBody.Result)
			}

			// Cleanup

			if err := testRuntime.DeletePolicy("test"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
