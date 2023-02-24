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

func executeRequest(w *httptest.ResponseRecorder, path string, method string, acceptEncoding string) {
	CompressHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := io.WriteString(w, requestBody)
		if err != nil {
			log.Fatalf("Error writing the request body: %v", err)
		}
	})).ServeHTTP(w, &http.Request{
		URL:    &url.URL{Path: path},
		Method: method,
		Header: http.Header{
			"Accept-Encoding": []string{acceptEncoding},
		},
	})
}

func TestCompressHandlerWithGzipOnInScopeEndpoints(t *testing.T) {
	endpoints := []string{"/v0/data", "/v1/data", "/v1/compile"}
	for _, endpoint := range endpoints {
		w := httptest.NewRecorder()
		executeRequest(w, endpoint, "POST", gzipEncoding)
		contentEncodingValue := w.Result().Header.Get("Content-Encoding")
		if contentEncodingValue != gzipEncoding {
			t.Errorf("wrong content encoding, got %q want %q", contentEncodingValue, gzipEncoding)
		}
		expectedLength := len(zipString(requestBody))
		receivedLength := w.Body.Len()
		if receivedLength != expectedLength {
			t.Errorf("wrong len, got %d want %d", w.Body.Len(), expectedLength)
		}
		receivedBody := unzip(w.Body.Bytes())
		if receivedBody != requestBody {
			t.Errorf("wrong body, got %v, want %v", receivedBody, requestBody)
		}
	}
}

func TestHandlerOnEndpointsWithoutCompression(t *testing.T) {
	w := httptest.NewRecorder()
	executeRequest(w, "/metrics", "GET", gzipEncoding)
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
	return string(plainOutput)
}
