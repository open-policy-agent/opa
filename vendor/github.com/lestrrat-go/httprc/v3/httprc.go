package httprc

import (
	"context"
	"net/http"
	"time"

	"github.com/lestrrat-go/httprc/v3/errsink"
	"github.com/lestrrat-go/httprc/v3/tracesink"
)

// Buffer size constants
const (
	// ReadBufferSize is the default buffer size for reading HTTP responses (10MB)
	ReadBufferSize = 1024 * 1024 * 10
	// MaxBufferSize is the maximum allowed buffer size (1GB)
	MaxBufferSize = 1024 * 1024 * 1000
)

// Client worker constants
const (
	// DefaultWorkers is the default number of worker goroutines
	DefaultWorkers = 5
)

// Interval constants
const (
	// DefaultMaxInterval is the default maximum interval between fetches (30 days)
	DefaultMaxInterval = 24 * time.Hour * 30
	// DefaultMinInterval is the default minimum interval between fetches (15 minutes)
	DefaultMinInterval = 15 * time.Minute
	// oneDay is used internally for time calculations
	oneDay = 24 * time.Hour
)

// utility to round up intervals to the nearest second
func roundupToSeconds(d time.Duration) time.Duration {
	if diff := d % time.Second; diff > 0 {
		return d + time.Second - diff
	}
	return d
}

// ErrorSink is an interface that abstracts a sink for errors.
type ErrorSink = errsink.Interface

type TraceSink = tracesink.Interface

// HTTPClient is an interface that abstracts a "net/http".Client, so that
// users can provide their own implementation of the HTTP client, if need be.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// Transformer is used to convert the body of an HTTP response into an appropriate
// object of type T.
type Transformer[T any] interface {
	Transform(context.Context, *http.Response) (T, error)
}

// TransformFunc is a function type that implements the Transformer interface.
type TransformFunc[T any] func(context.Context, *http.Response) (T, error)

func (f TransformFunc[T]) Transform(ctx context.Context, res *http.Response) (T, error) {
	return f(ctx, res)
}

// Resource is a single resource that can be retrieved via HTTP, and (possibly) transformed
// into an arbitrary object type.
//
// Realistically, there is no need for third-parties to implement this interface. This exists
// to provide a way to aggregate `httprc.ResourceBase` objects with different specialized types
// into a single collection.
//
// See ResourceBase for details
type Resource interface { //nolint:interfacebloat
	Get(any) error
	Next() time.Time
	SetNext(time.Time)
	URL() string
	Sync(context.Context) error
	ConstantInterval() time.Duration
	MaxInterval() time.Duration
	SetMaxInterval(time.Duration)
	MinInterval() time.Duration
	SetMinInterval(time.Duration)
	IsBusy() bool
	SetBusy(bool)
	Ready(context.Context) error
}
