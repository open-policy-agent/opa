package handlers

import (
	"bytes"
	"compress/gzip"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

const (
	gzipEncoding = "gzip"
	requestBody  = "Hello World!\n"
)

var defaultCompressionLevel = gzip.BestCompression

type compressHandlerTestScenario struct {
	path                       string
	method                     string
	acceptEncoding             string
	gzipMinSize                int
	expectedCompressedResponse bool
}

func executeRequest(w *httptest.ResponseRecorder, testScenario compressHandlerTestScenario) {
	CompressHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := io.WriteString(w, requestBody)
		if err != nil {
			log.Fatalf("Error writing the request body: %v", err)
		}
	}), testScenario.gzipMinSize, defaultCompressionLevel).ServeHTTP(w, &http.Request{
		URL:    &url.URL{Path: testScenario.path},
		Method: testScenario.method,
		Header: http.Header{
			"Accept-Encoding": []string{testScenario.acceptEncoding},
		},
	})
}

func TestCompressHandlerWithGzipOnInScopeEndpoints(t *testing.T) {
	tests := map[string]compressHandlerTestScenario{
		"v0PostDataCompressed": {
			path:                       "/v0/data",
			method:                     "POST",
			acceptEncoding:             gzipEncoding,
			gzipMinSize:                1,
			expectedCompressedResponse: true,
		},
		"v1PostDataCompressed": {
			path:                       "/v1/data",
			method:                     "POST",
			acceptEncoding:             gzipEncoding,
			gzipMinSize:                1,
			expectedCompressedResponse: true,
		},
		"v1PostCompileCompressed": {
			path:                       "/v1/compile",
			method:                     "POST",
			acceptEncoding:             gzipEncoding,
			gzipMinSize:                1,
			expectedCompressedResponse: true,
		},
		"v0PostDataUncompressed": {
			path:                       "/v0/data",
			method:                     "POST",
			acceptEncoding:             gzipEncoding,
			gzipMinSize:                1024,
			expectedCompressedResponse: false,
		},
		"v1PostDataUncompressed": {
			path:                       "/v1/data",
			method:                     "POST",
			acceptEncoding:             gzipEncoding,
			gzipMinSize:                1024,
			expectedCompressedResponse: false,
		},
		"v1PostCompileUncompressed": {
			path:                       "/v1/compile",
			method:                     "POST",
			acceptEncoding:             gzipEncoding,
			gzipMinSize:                1024,
			expectedCompressedResponse: false,
		},
		"v1PostCompileAcceptEncodingAll": {
			path:                       "/v1/compile",
			method:                     "POST",
			acceptEncoding:             "*/*",
			gzipMinSize:                1024,
			expectedCompressedResponse: false,
		},
	}
	for name, ts := range tests {
		w := httptest.NewRecorder()
		executeRequest(w, ts)
		if w.Result().Header.Get("Vary") != "Accept-Encoding" {
			t.Error("missing the Vary header")
		}
		contentEncodingValue := w.Result().Header.Get("Content-Encoding")
		if ts.expectedCompressedResponse {
			if contentEncodingValue != gzipEncoding {
				t.Errorf("wrong content encoding, got %q want %q", contentEncodingValue, gzipEncoding)
			}
			expectedLength := len(zipString(requestBody))
			receivedLength := w.Body.Len()
			if receivedLength != expectedLength {
				t.Errorf("test: %s wrong len, got %d want %d", name, w.Body.Len(), expectedLength)
			}
			receivedBody := unzip(w.Body.Bytes())
			if receivedBody != requestBody {
				t.Errorf("test: %s wrong body, got %v, want %v", name, receivedBody, requestBody)
			}
		} else {
			if contentEncodingValue == gzipEncoding {
				t.Errorf("wrong content encoding, got %q want %q", contentEncodingValue, "")
			}
			expectedLength := len(requestBody)
			receivedLength := w.Body.Len()
			if receivedLength != expectedLength {
				t.Errorf("test: %s wrong len, got %d want %d", name, w.Body.Len(), expectedLength)
			}
			receivedBody := w.Body.String()
			if receivedBody != requestBody {
				t.Errorf("test: %s wrong body, got %v, want %v", name, receivedBody, requestBody)
			}
		}
	}
}

func TestHandlerOnEndpointsWithoutCompression(t *testing.T) {
	testScenario := compressHandlerTestScenario{
		path:           "/metrics",
		method:         "GET",
		acceptEncoding: gzipEncoding,
		gzipMinSize:    1,
	}
	w := httptest.NewRecorder()
	executeRequest(w, testScenario)
	contentEncodingValue := w.Result().Header.Get("Content-Encoding")
	if contentEncodingValue != "" {
		t.Errorf("wrong content encoding, got %q want %q", contentEncodingValue, gzipEncoding)
	}

	expectedLength := len(requestBody)
	receivedLength := w.Body.Len()
	if receivedLength != expectedLength {
		t.Errorf("wrong len, got %d want %d", w.Body.Len(), expectedLength)
	}
}

func zipString(input string) []byte {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write([]byte(input)); err != nil {
		log.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		log.Fatal(err)
	}
	return b.Bytes()
}

func unzip(body []byte) string {
	reader := bytes.NewReader(body)
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		log.Fatalf("Unexpected gzip error: %v", err)
	}
	plainOutput, err := io.ReadAll(gzReader)
	if err != nil {
		log.Fatalf("Unexpected gzip error: %v", err)
	}
	err = gzReader.Close()
	if err != nil {
		log.Fatalf("Unexpected gzip close err: %v", err)
	}
	return string(plainOutput)
}
