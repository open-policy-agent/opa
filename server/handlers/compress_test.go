package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

var contentType = "text/plain; charset=utf-8"

func executeRequest(w *httptest.ResponseRecorder, acceptEncoding string) {
	CompressHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(9*1024))
		w.Header().Set("Content-Type", contentType)
		for i := 0; i < 1024; i++ {
			io.WriteString(w, "Hello World!\n")
		}
	})).ServeHTTP(w, &http.Request{
		URL:    &url.URL{Path: "/v1/data"},
		Method: "GET",
		Header: http.Header{
			"Accept-Encoding": []string{acceptEncoding},
		},
	})
}

func TestCompressHandlerWithGzip(t *testing.T) {
	w := httptest.NewRecorder()
	executeRequest(w, "gzip")
	if w.Result().Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("wrong content encoding, got %q want %q", w.HeaderMap.Get("Content-Encoding"), "gzip")
	}
	if w.Result().Header.Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Errorf("wrong content type, got %s want %s", w.HeaderMap.Get("Content-Type"), "text/plain; charset=utf-8")
	}
	//expectedLength := len("Hello World!") * 1024
	//if w.Body.Len() != expectedLength {
	//	t.Errorf("wrong len, got %d want %d", w.Body.Len(), expectedLength)
	//}
	//if l := w.Result().Header.Get("Content-Length"); l != "" {
	//	t.Errorf("wrong content-length. got %q expected %q", l, "")
	//}
}
