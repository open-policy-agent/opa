package httprc

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/lestrrat-go/httprc/v3/errsink"
	"github.com/lestrrat-go/httprc/v3/proxysink"
	"github.com/lestrrat-go/httprc/v3/tracesink"
)

// setupSink creates and starts a proxy for the given sink if it's not a Nop sink
// Returns the sink to use and a cancel function that should be chained with the original cancel
func setupSink[T any, S proxysink.Backend[T], NopType any](ctx context.Context, sink S, wg *sync.WaitGroup) (S, context.CancelFunc) {
	if _, ok := any(sink).(NopType); ok {
		return sink, func() {}
	}

	proxy := proxysink.New[T](sink)
	wg.Add(1)
	go func(ctx context.Context, wg *sync.WaitGroup, proxy *proxysink.Proxy[T]) {
		defer wg.Done()
		proxy.Run(ctx)
	}(ctx, wg, proxy)

	// proxy can be converted to one of the sink subtypes
	s, ok := any(proxy).(S)
	if !ok {
		panic("type assertion failed: proxy cannot be converted to type S")
	}
	return s, proxy.Close
}

// Client is the main entry point for the httprc package.
type Client struct {
	mu                 sync.Mutex
	httpcl             HTTPClient
	numWorkers         int
	running            bool
	errSink            ErrorSink
	traceSink          TraceSink
	wl                 Whitelist
	defaultMaxInterval time.Duration
	defaultMinInterval time.Duration
}

// NewClient creates a new `httprc.Client` object.
//
// By default ALL urls are allowed. This may not be suitable for you if
// are using this in a production environment. You are encouraged to specify
// a whitelist using the `WithWhitelist` option.
func NewClient(options ...NewClientOption) *Client {
	//nolint:staticcheck
	var errSink ErrorSink = errsink.NewNop()
	//nolint:staticcheck
	var traceSink TraceSink = tracesink.NewNop()
	var wl Whitelist = InsecureWhitelist{}
	var httpcl HTTPClient = http.DefaultClient

	defaultMinInterval := DefaultMinInterval
	defaultMaxInterval := DefaultMaxInterval

	numWorkers := DefaultWorkers
	//nolint:forcetypeassert
	for _, option := range options {
		switch option.Ident() {
		case identHTTPClient{}:
			httpcl = option.Value().(HTTPClient)
		case identWorkers{}:
			numWorkers = option.Value().(int)
		case identErrorSink{}:
			errSink = option.Value().(ErrorSink)
		case identTraceSink{}:
			traceSink = option.Value().(TraceSink)
		case identWhitelist{}:
			wl = option.Value().(Whitelist)
		}
	}

	if numWorkers <= 0 {
		numWorkers = 1
	}
	return &Client{
		httpcl:     httpcl,
		numWorkers: numWorkers,
		errSink:    errSink,
		traceSink:  traceSink,
		wl:         wl,

		defaultMinInterval: defaultMinInterval,
		defaultMaxInterval: defaultMaxInterval,
	}
}

// Start sets the client into motion. It will start a number of worker goroutines,
// and return a Controller object that you can use to control the execution of
// the client.
//
// If you attempt to call Start more than once, it will return an error.
func (c *Client) Start(octx context.Context) (Controller, error) {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return nil, errAlreadyRunning
	}
	c.running = true
	c.mu.Unlock()

	// DON'T CANCEL THIS IN THIS METHOD! It's the responsibility of the
	// controller to cancel this context.
	ctx, cancel := context.WithCancel(octx)

	var donewg sync.WaitGroup

	// start proxy goroutines that will accept sink requests
	// and forward them to the appropriate sink
	errSink, errCancel := setupSink[error, ErrorSink, errsink.Nop](ctx, c.errSink, &donewg)
	traceSink, traceCancel := setupSink[string, TraceSink, tracesink.Nop](ctx, c.traceSink, &donewg)

	// Chain the cancel functions
	ocancel := cancel
	cancel = func() {
		ocancel()
		errCancel()
		traceCancel()
	}

	chbuf := c.numWorkers + 1
	incoming := make(chan any, chbuf)
	outgoing := make(chan Resource, chbuf)
	syncoutgoing := make(chan synchronousRequest, chbuf)

	var readywg sync.WaitGroup
	readywg.Add(c.numWorkers)
	donewg.Add(c.numWorkers)
	for range c.numWorkers {
		wrk := worker{
			incoming:  incoming,
			next:      outgoing,
			nextsync:  syncoutgoing,
			errSink:   errSink,
			traceSink: traceSink,
			httpcl:    c.httpcl,
		}
		go wrk.Run(ctx, &readywg, &donewg)
	}

	tickInterval := oneDay
	ctrl := &controller{
		cancel:    cancel,
		incoming:  incoming,
		shutdown:  make(chan struct{}),
		traceSink: traceSink,
		wl:        c.wl,
	}

	backend := &ctrlBackend{
		items:        make(map[string]Resource),
		outgoing:     outgoing,
		syncoutgoing: syncoutgoing,
		incoming:     incoming,
		traceSink:    traceSink,
		tickInterval: tickInterval,
		check:        time.NewTicker(tickInterval),

		defaultMinInterval: c.defaultMinInterval,
		defaultMaxInterval: c.defaultMaxInterval,
	}
	donewg.Add(1)
	readywg.Add(1)
	go backend.loop(ctx, &readywg, &donewg)

	go func(wg *sync.WaitGroup, ch chan struct{}) {
		wg.Wait()
		close(ch)
	}(&donewg, ctrl.shutdown)

	readywg.Wait()

	return ctrl, nil
}
