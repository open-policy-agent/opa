package httprc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lestrrat-go/blackmagic"
	"github.com/lestrrat-go/httpcc"
	"github.com/lestrrat-go/httprc/v3/tracesink"
)

// ResourceBase is a generic Resource type
type ResourceBase[T any] struct {
	u           string
	ready       chan struct{} // closed when the resource is ready (i.e. after first successful fetch)
	once        sync.Once
	httpcl      HTTPClient
	t           Transformer[T]
	r           atomic.Value
	next        atomic.Value
	interval    time.Duration
	minInterval atomic.Int64
	maxInterval atomic.Int64
	busy        atomic.Bool
}

// NewResource creates a new Resource object which after fetching the
// resource from the URL, will transform the response body using the
// provided Transformer to an object of type T.
//
// This function will return an error if the URL is not a valid URL
// (i.e. it cannot be parsed by url.Parse), or if the transformer is nil.
func NewResource[T any](s string, transformer Transformer[T], options ...NewResourceOption) (*ResourceBase[T], error) {
	var httpcl HTTPClient
	var interval time.Duration
	minInterval := DefaultMinInterval
	maxInterval := DefaultMaxInterval
	//nolint:forcetypeassert
	for _, option := range options {
		switch option.Ident() {
		case identHTTPClient{}:
			httpcl = option.Value().(HTTPClient)
		case identMinimumInterval{}:
			minInterval = option.Value().(time.Duration)
		case identMaximumInterval{}:
			maxInterval = option.Value().(time.Duration)
		case identConstantInterval{}:
			interval = option.Value().(time.Duration)
		}
	}
	if transformer == nil {
		return nil, fmt.Errorf(`httprc.NewResource: %w`, errTransformerRequired)
	}

	if s == "" {
		return nil, fmt.Errorf(`httprc.NewResource: %w`, errURLCannotBeEmpty)
	}

	if _, err := url.Parse(s); err != nil {
		return nil, fmt.Errorf(`httprc.NewResource: %w`, err)
	}
	r := &ResourceBase[T]{
		u:        s,
		httpcl:   httpcl,
		t:        transformer,
		interval: interval,
		ready:    make(chan struct{}),
	}
	if httpcl != nil {
		r.httpcl = httpcl
	}
	r.minInterval.Store(int64(minInterval))
	r.maxInterval.Store(int64(maxInterval))
	r.SetNext(time.Unix(0, 0)) // initially, it should be fetched immediately
	return r, nil
}

// URL returns the URL of the resource.
func (r *ResourceBase[T]) URL() string {
	return r.u
}

// Ready returns an empty error when the resource is ready. If the context
// is canceled before the resource is ready, it will return the error from
// the context.
func (r *ResourceBase[T]) Ready(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.ready:
		return nil
	}
}

// Get assigns the value of the resource to the provided pointer.
// If using the `httprc.ResourceBase[T]` type directly, you can use the `Resource()`
// method to get the resource directly.
//
// This method exists because parametric types cannot be assigned to a single object type
// that return different return values of the specialized type. i.e. for resources
// `ResourceBase[A]` and `ResourceBase[B]`, we cannot have a single interface that can
// be assigned to the same interface type `X` that expects a `Resource()` method that
// returns `A` or `B` depending on the type of the resource. When accessing the
// resource through the `httprc.Resource` interface, use this method to obtain the
// stored value.
func (r *ResourceBase[T]) Get(dst interface{}) error {
	return blackmagic.AssignIfCompatible(dst, r.Resource())
}

// Resource returns the last fetched resource. If the resource has not been
// fetched yet, this will return the zero value of type T.
//
// If you would rather wait until the resource is fetched, you can use the
// `Ready()` method to wait until the resource is ready (i.e. fetched at least once).
func (r *ResourceBase[T]) Resource() T {
	v := r.r.Load()
	switch v := v.(type) {
	case T:
		return v
	default:
		var zero T
		return zero
	}
}

func (r *ResourceBase[T]) Next() time.Time {
	//nolint:forcetypeassert
	return r.next.Load().(time.Time)
}

func (r *ResourceBase[T]) SetNext(v time.Time) {
	r.next.Store(v)
}

func (r *ResourceBase[T]) ConstantInterval() time.Duration {
	return r.interval
}

func (r *ResourceBase[T]) MaxInterval() time.Duration {
	return time.Duration(r.maxInterval.Load())
}

func (r *ResourceBase[T]) MinInterval() time.Duration {
	return time.Duration(r.minInterval.Load())
}

func (r *ResourceBase[T]) SetMaxInterval(v time.Duration) {
	r.maxInterval.Store(int64(v))
}

func (r *ResourceBase[T]) SetMinInterval(v time.Duration) {
	r.minInterval.Store(int64(v))
}

func (r *ResourceBase[T]) SetBusy(v bool) {
	r.busy.Store(v)
}

func (r *ResourceBase[T]) IsBusy() bool {
	return r.busy.Load()
}

// limitedBody is a wrapper around an io.Reader that will only read up to
// MaxBufferSize bytes. This is provided to prevent the user from accidentally
// reading a huge response body into memory
type limitedBody struct {
	rdr   io.Reader
	close func() error
}

func (l *limitedBody) Read(p []byte) (n int, err error) {
	return l.rdr.Read(p)
}

func (l *limitedBody) Close() error {
	return l.close()
}

type traceSinkKey struct{}

func withTraceSink(ctx context.Context, sink TraceSink) context.Context {
	return context.WithValue(ctx, traceSinkKey{}, sink)
}

func traceSinkFromContext(ctx context.Context) TraceSink {
	if v := ctx.Value(traceSinkKey{}); v != nil {
		//nolint:forcetypeassert
		return v.(TraceSink)
	}
	return tracesink.Nop{}
}

type httpClientKey struct{}

func withHTTPClient(ctx context.Context, cl HTTPClient) context.Context {
	return context.WithValue(ctx, httpClientKey{}, cl)
}

func httpClientFromContext(ctx context.Context) HTTPClient {
	if v := ctx.Value(httpClientKey{}); v != nil {
		//nolint:forcetypeassert
		return v.(HTTPClient)
	}
	return http.DefaultClient
}

func (r *ResourceBase[T]) Sync(ctx context.Context) error {
	traceSink := traceSinkFromContext(ctx)
	httpcl := r.httpcl
	if httpcl == nil {
		httpcl = httpClientFromContext(ctx)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.u, nil)
	if err != nil {
		return fmt.Errorf(`httprc.Resource.Sync: failed to create request: %w`, err)
	}

	traceSink.Put(ctx, fmt.Sprintf("httprc.Resource.Sync: fetching %q", r.u))
	res, err := httpcl.Do(req)
	if err != nil {
		return fmt.Errorf(`httprc.Resource.Sync: failed to execute HTTP request: %w`, err)
	}
	defer res.Body.Close()

	next := r.calculateNextRefreshTime(ctx, res)
	traceSink.Put(ctx, fmt.Sprintf("httprc.Resource.Sync: next refresh time for %q is %v", r.u, next))
	r.SetNext(next)

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf(`httprc.Resource.Sync: %w (status code=%d, url=%q)`, errUnexpectedStatusCode, res.StatusCode, r.u)
	}

	// replace the body of the response with a limited reader that
	// will only read up to MaxBufferSize bytes
	res.Body = &limitedBody{
		rdr:   &io.LimitedReader{R: res.Body, N: MaxBufferSize},
		close: res.Body.Close,
	}
	traceSink.Put(ctx, fmt.Sprintf("httprc.Resource.Sync: transforming %q", r.u))
	v, err := r.transform(ctx, res)
	if err != nil {
		return fmt.Errorf(`httprc.Resource.Sync: %w: %w`, errTransformerFailed, err)
	}

	traceSink.Put(ctx, fmt.Sprintf("httprc.Resource.Sync: storing new value for %q", r.u))
	r.r.Store(v)
	r.once.Do(func() { close(r.ready) })
	traceSink.Put(ctx, fmt.Sprintf("httprc.Resource.Sync: stored value for %q", r.u))
	return nil
}

func (r *ResourceBase[T]) transform(ctx context.Context, res *http.Response) (ret T, gerr error) {
	// Protect the call to Transform with a defer/recover block, so that even
	// if the Transform method panics, we can recover from it and return an error
	defer func() {
		if recovered := recover(); recovered != nil {
			gerr = fmt.Errorf(`httprc.Resource.transform: %w: %v`, errRecoveredFromPanic, recovered)
		}
	}()
	return r.t.Transform(ctx, res)
}

func (r *ResourceBase[T]) determineNextFetchInterval(ctx context.Context, name string, fromHeader, minValue, maxValue time.Duration) time.Duration {
	traceSink := traceSinkFromContext(ctx)

	if fromHeader > maxValue {
		traceSink.Put(ctx, fmt.Sprintf("httprc.Resource.Sync: %s %s > maximum interval, using maximum interval %s", r.URL(), name, maxValue))
		return maxValue
	}

	if fromHeader < minValue {
		traceSink.Put(ctx, fmt.Sprintf("httprc.Resource.Sync: %s %s < minimum interval, using minimum interval %s", r.URL(), name, minValue))
		return minValue
	}

	traceSink.Put(ctx, fmt.Sprintf("httprc.Resource.Sync: %s Using %s (%s)", r.URL(), name, fromHeader))
	return fromHeader
}

func (r *ResourceBase[T]) calculateNextRefreshTime(ctx context.Context, res *http.Response) time.Time {
	traceSink := traceSinkFromContext(ctx)
	now := time.Now()

	// If constant interval is set, use that regardless of what the
	// response headers say.
	if interval := r.ConstantInterval(); interval > 0 {
		traceSink.Put(ctx, fmt.Sprintf("httprc.Resource.Sync: %s Explicit interval set, using value %s", r.URL(), interval))
		return now.Add(interval)
	}

	if interval := r.extractCacheControlMaxAge(ctx, res); interval > 0 {
		return now.Add(interval)
	}

	if interval := r.extractExpiresInterval(ctx, res); interval > 0 {
		return now.Add(interval)
	}

	traceSink.Put(ctx, fmt.Sprintf("httprc.Resource.Sync: %s No cache-control/expires headers found, using minimum interval", r.URL()))
	return now.Add(r.MinInterval())
}

func (r *ResourceBase[T]) extractCacheControlMaxAge(ctx context.Context, res *http.Response) time.Duration {
	traceSink := traceSinkFromContext(ctx)

	v := res.Header.Get(`Cache-Control`)
	if v == "" {
		return 0
	}

	dir, err := httpcc.ParseResponse(v)
	if err != nil {
		return 0
	}

	maxAge, ok := dir.MaxAge()
	if !ok {
		return 0
	}

	traceSink.Put(ctx, fmt.Sprintf("httprc.Resource.Sync: %s Cache-Control=max-age directive set (%d)", r.URL(), maxAge))
	return r.determineNextFetchInterval(
		ctx,
		"max-age",
		time.Duration(maxAge)*time.Second,
		r.MinInterval(),
		r.MaxInterval(),
	)
}

func (r *ResourceBase[T]) extractExpiresInterval(ctx context.Context, res *http.Response) time.Duration {
	traceSink := traceSinkFromContext(ctx)

	v := res.Header.Get(`Expires`)
	if v == "" {
		return 0
	}

	expires, err := http.ParseTime(v)
	if err != nil {
		return 0
	}

	traceSink.Put(ctx, fmt.Sprintf("httprc.Resource.Sync: %s Expires header set (%s)", r.URL(), expires))
	return r.determineNextFetchInterval(
		ctx,
		"expires",
		time.Until(expires),
		r.MinInterval(),
		r.MaxInterval(),
	)
}
