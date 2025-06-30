package httprc

import (
	"context"
	"fmt"
	"time"
)

type Controller interface {
	// Add adds a new `http.Resource` to the controller. If the resource already exists,
	// it will return an error.
	Add(context.Context, Resource, ...AddOption) error

	// Lookup a `httprc.Resource` by its URL. If the resource does not exist, it
	// will return an error.
	Lookup(context.Context, string) (Resource, error)

	// Remove a `httprc.Resource` from the controller by its URL. If the resource does
	// not exist, it will return an error.
	Remove(context.Context, string) error

	// Refresh forces a resource to be refreshed immediately. If the resource does
	// not exist, or if the refresh fails, it will return an error.
	Refresh(context.Context, string) error

	ShutdownContext(context.Context) error
	Shutdown(time.Duration) error
}

type controller struct {
	cancel    context.CancelFunc
	incoming  chan any // incoming requests to the controller
	shutdown  chan struct{}
	traceSink TraceSink
	wl        Whitelist
}

// Shutdown is a convenience function that calls ShutdownContext with a
// context that has a timeout of `timeout`.
func (c *controller) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return c.ShutdownContext(ctx)
}

// ShutdownContext stops the client and all associated goroutines, and waits for them
// to finish. If the context is canceled, the function will return immediately:
// there fore you should not use the context you used to start the client (because
// presumably it's already canceled).
//
// Waiting for the client shutdown will also ensure that all sinks are properly
// flushed.
func (c *controller) ShutdownContext(ctx context.Context) error {
	c.cancel()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.shutdown:
		return nil
	}
}

type ctrlRequest[T any] struct {
	reply    chan T
	resource Resource
	u        string
}
type addRequest ctrlRequest[backendResponse[struct{}]]
type rmRequest ctrlRequest[backendResponse[struct{}]]
type refreshRequest ctrlRequest[backendResponse[struct{}]]
type lookupRequest ctrlRequest[backendResponse[Resource]]
type synchronousRequest ctrlRequest[backendResponse[struct{}]]
type adjustIntervalRequest struct {
	resource Resource
}

type backendResponse[T any] struct {
	payload T
	err     error
}

func sendBackend[TReq any, TB any](ctx context.Context, backendCh chan any, v TReq, replyCh chan backendResponse[TB]) (TB, error) {
	select {
	case <-ctx.Done():
	case backendCh <- v:
	}

	select {
	case <-ctx.Done():
		var zero TB
		return zero, ctx.Err()
	case res := <-replyCh:
		return res.payload, res.err
	}
}

// Lookup returns a resource by its URL. If the resource does not exist, it
// will return an error.
//
// Unfortunately, due to the way typed parameters are handled in Go, we can only
// return a Resource object (and not a ResourceBase[T] object). This means that
// you will either need to use the `Resource.Get()` method or use a type
// assertion to obtain a `ResourceBase[T]` to get to the actual object you are
// looking for
func (c *controller) Lookup(ctx context.Context, u string) (Resource, error) {
	reply := make(chan backendResponse[Resource], 1)
	req := lookupRequest{
		reply: reply,
		u:     u,
	}
	return sendBackend[lookupRequest, Resource](ctx, c.incoming, req, reply)
}

// Add adds a new resource to the controller. If the resource already
// exists, it will return an error.
//
// By default this function will automatically wait for the resource to be
// fetched once (by calling `r.Ready()`). Note that the `r.Ready()` call will NOT
// timeout unless you configure your context object with `context.WithTimeout`.
// To disable waiting, you can specify the `WithWaitReady(false)` option.
func (c *controller) Add(ctx context.Context, r Resource, options ...AddOption) error {
	c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: START Add(%q)", r.URL()))
	defer c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: END   Add(%q)", r.URL()))
	waitReady := true
	//nolint:forcetypeassert
	for _, option := range options {
		switch option.Ident() {
		case identWaitReady{}:
			waitReady = option.(addOption).Value().(bool)
		}
	}

	if !c.wl.IsAllowed(r.URL()) {
		return fmt.Errorf(`httprc.Controller.AddResource: cannot add %q: %w`, r.URL(), errBlockedByWhitelist)
	}

	reply := make(chan backendResponse[struct{}], 1)
	req := addRequest{
		reply:    reply,
		resource: r,
	}
	c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: sending add request for %q to backend", r.URL()))
	if _, err := sendBackend[addRequest, struct{}](ctx, c.incoming, req, reply); err != nil {
		return err
	}

	if waitReady {
		c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: waiting for resource %q to be ready", r.URL()))
		if err := r.Ready(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Remove removes a resource from the controller. If the resource does
// not exist, it will return an error.
func (c *controller) Remove(ctx context.Context, u string) error {
	reply := make(chan backendResponse[struct{}], 1)
	req := rmRequest{
		reply: reply,
		u:     u,
	}
	if _, err := sendBackend[rmRequest, struct{}](ctx, c.incoming, req, reply); err != nil {
		return err
	}
	return nil
}

// Refresh forces a resource to be refreshed immediately. If the resource does
// not exist, or if the refresh fails, it will return an error.
//
// This function is synchronous, and will block until the resource has been refreshed.
func (c *controller) Refresh(ctx context.Context, u string) error {
	reply := make(chan backendResponse[struct{}], 1)
	req := refreshRequest{
		reply: reply,
		u:     u,
	}

	if _, err := sendBackend[refreshRequest, struct{}](ctx, c.incoming, req, reply); err != nil {
		return err
	}
	return nil
}
