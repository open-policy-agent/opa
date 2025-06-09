package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

func BenchmarkDataPostV1Request(b *testing.B) {
	f := newBenchFixture(b)
	err := f.v1(http.MethodPut, "/policies/test", `package test

default hello := false

hello if input.message == "world"
`, 200, "")
	if err != nil {
		b.Fatal(err)
	}

	for i := range b.N {
		req := newReqV1("POST", "/data/test", `{"input": {"message": "world"}}`)

		err := f.executeRequest(req, 200, "{\"result\":{\"hello\":true}}\n")
		if err != nil {
			b.Fatalf("Unexpected error from POST /data/test: %v, iteration: %d", err, i)
		}
	}
}

func newBenchFixture(b *testing.B, opts ...any) *fixture {
	ctx := context.Background()
	server := New().
		WithAddresses([]string{"localhost:8182"}).
		WithStore(inmem.New()) // potentially overridden via opts

	for _, opt := range opts {
		if opt, ok := opt.(func(*Server)); ok {
			opt(server)
		}
	}

	var mOpts []func(*plugins.Manager)
	for _, opt := range opts {
		if opt, ok := opt.(func(*plugins.Manager)); ok {
			mOpts = append(mOpts, opt)
		}
	}

	m, err := plugins.New([]byte{}, "test", server.store, mOpts...)
	if err != nil {
		b.Fatal(err)
	}
	server = server.WithManager(m)
	if err := m.Start(ctx); err != nil {
		b.Fatal(err)
	}
	server, err = server.Init(ctx)
	if err != nil {
		b.Fatal(err)
	}
	recorder := httptest.NewRecorder()

	return &fixture{
		server:   server,
		recorder: recorder,
	}
}
