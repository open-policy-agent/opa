package httprc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
)

type bytesTransformer struct{}

// BytesTransformer returns a Transformer that reads the entire response body
// as a byte slice. This is the default Transformer used by httprc.Client
func BytesTransformer() Transformer[[]byte] {
	return bytesTransformer{}
}

func (bytesTransformer) Transform(_ context.Context, res *http.Response) ([]byte, error) {
	return io.ReadAll(res.Body)
}

type jsonTransformer[T any] struct{}

// JSONTransformer returns a Transformer that decodes the response body as JSON
// into the provided type T.
func JSONTransformer[T any]() Transformer[T] {
	return jsonTransformer[T]{}
}

func (jsonTransformer[T]) Transform(_ context.Context, res *http.Response) (T, error) {
	var v T
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil {
		var zero T
		return zero, err
	}
	return v, nil
}
