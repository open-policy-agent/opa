package httprc

import (
	"context"
	"fmt"
	"sync"
	"time"
)

func (c *ctrlBackend) adjustInterval(ctx context.Context, req adjustIntervalRequest) {
	interval := roundupToSeconds(time.Until(req.resource.Next()))
	c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: got adjust request (current tick interval=%s, next for %q=%s)", c.tickInterval, req.resource.URL(), interval))

	if interval < time.Second {
		interval = time.Second
	}

	if c.tickInterval < interval {
		c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: no adjusting required (time to next check %s > current tick interval %s)", interval, c.tickInterval))
	} else {
		c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: adjusting tick interval to %s", interval))
		c.tickInterval = interval
		c.check.Reset(interval)
	}
}

func (c *ctrlBackend) addResource(ctx context.Context, req addRequest) {
	r := req.resource
	if _, ok := c.items[r.URL()]; ok {
		// Already exists
		sendReply(ctx, req.reply, struct{}{}, errResourceAlreadyExists)
		return
	}
	c.items[r.URL()] = r

	if r.MaxInterval() == 0 {
		r.SetMaxInterval(c.defaultMaxInterval)
	}

	if r.MinInterval() == 0 {
		c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: set minimum interval to %s", c.defaultMinInterval))
		r.SetMinInterval(c.defaultMinInterval)
	}

	c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: added resource %q", r.URL()))
	sendReply(ctx, req.reply, struct{}{}, nil)
	c.SetTickInterval(time.Nanosecond)
}

func (c *ctrlBackend) rmResource(ctx context.Context, req rmRequest) {
	u := req.u
	if _, ok := c.items[u]; !ok {
		sendReply(ctx, req.reply, struct{}{}, errResourceNotFound)
		return
	}

	delete(c.items, u)

	minInterval := oneDay
	for _, item := range c.items {
		if d := item.MinInterval(); d < minInterval {
			minInterval = d
		}
	}

	close(req.reply)
	c.check.Reset(minInterval)
}

func (c *ctrlBackend) refreshResource(ctx context.Context, req refreshRequest) {
	c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: [refresh] START %q", req.u))
	defer c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: [refresh] END   %q", req.u))
	u := req.u
	r, ok := c.items[u]
	if !ok {
		c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: [refresh] %s is not registered", req.u))
		sendReply(ctx, req.reply, struct{}{}, errResourceNotFound)
		return
	}

	// Make sure it's ready
	if err := r.Ready(ctx); err != nil {
		c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: [refresh] %s did not become ready: %v", req.u, err))
		sendReply(ctx, req.reply, struct{}{}, err)
		return
	}

	r.SetNext(time.Unix(0, 0))
	sendWorkerSynchronous(ctx, c.syncoutgoing, synchronousRequest{
		resource: r,
		reply:    req.reply,
	})
	c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: [refresh] sync request for %s sent to worker pool", req.u))
}

func (c *ctrlBackend) lookupResource(ctx context.Context, req lookupRequest) {
	u := req.u
	r, ok := c.items[u]
	if !ok {
		sendReply(ctx, req.reply, nil, errResourceNotFound)
		return
	}
	sendReply(ctx, req.reply, r, nil)
}

func (c *ctrlBackend) handleRequest(ctx context.Context, req any) {
	switch req := req.(type) {
	case adjustIntervalRequest:
		c.adjustInterval(ctx, req)
	case addRequest:
		c.addResource(ctx, req)
	case rmRequest:
		c.rmResource(ctx, req)
	case refreshRequest:
		c.refreshResource(ctx, req)
	case lookupRequest:
		c.lookupResource(ctx, req)
	default:
		c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: unknown request type %T", req))
	}
}

func sendWorker(ctx context.Context, ch chan Resource, r Resource) {
	r.SetBusy(true)
	select {
	case <-ctx.Done():
	case ch <- r:
	}
}

func sendWorkerSynchronous(ctx context.Context, ch chan synchronousRequest, r synchronousRequest) {
	r.resource.SetBusy(true)
	select {
	case <-ctx.Done():
	case ch <- r:
	}
}

func sendReply[T any](ctx context.Context, ch chan backendResponse[T], v T, err error) {
	defer close(ch)
	select {
	case <-ctx.Done():
	case ch <- backendResponse[T]{payload: v, err: err}:
	}
}

type ctrlBackend struct {
	items              map[string]Resource
	outgoing           chan Resource
	syncoutgoing       chan synchronousRequest
	incoming           chan any // incoming requests to the controller
	traceSink          TraceSink
	tickInterval       time.Duration
	check              *time.Ticker
	defaultMaxInterval time.Duration
	defaultMinInterval time.Duration
}

func (c *ctrlBackend) loop(ctx context.Context, readywg, donewg *sync.WaitGroup) {
	c.traceSink.Put(ctx, "httprc controller: starting main controller loop")
	readywg.Done()
	defer c.traceSink.Put(ctx, "httprc controller: stopping main controller loop")
	defer donewg.Done()
	for {
		c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: waiting for request or tick (tick interval=%s)", c.tickInterval))
		select {
		case req := <-c.incoming:
			c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: got request %T", req))
			c.handleRequest(ctx, req)
		case t := <-c.check.C:
			c.periodicCheck(ctx, t)
		case <-ctx.Done():
			return
		}
	}
}

func (c *ctrlBackend) periodicCheck(ctx context.Context, t time.Time) {
	c.traceSink.Put(ctx, "httprc controller: START periodic check")
	defer c.traceSink.Put(ctx, "httprc controller: END periodic check")
	var minNext time.Time
	var dispatched int
	minInterval := -1 * time.Second
	for _, item := range c.items {
		c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: checking resource %q", item.URL()))

		next := item.Next()
		if minNext.IsZero() || next.Before(minNext) {
			minNext = next
		}

		if interval := item.MinInterval(); minInterval < 0 || interval < minInterval {
			minInterval = interval
		}

		c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: resource %q isBusy=%t, next(%s).After(%s)=%t", item.URL(), item.IsBusy(), next, t, next.After(t)))
		if item.IsBusy() || next.After(t) {
			c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: resource %q is busy or not ready yet, skipping", item.URL()))
			continue
		}
		c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: resource %q is ready, dispatching to worker pool", item.URL()))

		dispatched++
		c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: dispatching resource %q to worker pool", item.URL()))
		sendWorker(ctx, c.outgoing, item)
	}

	c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: dispatched %d resources", dispatched))

	// Next check is always at the earliest next check + 1 second.
	// The extra second makes sure that we are _past_ the actual next check time
	// so we can send the resource to the worker pool
	if interval := time.Until(minNext); interval > 0 {
		c.SetTickInterval(roundupToSeconds(interval) + time.Second)
		c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: resetting check intervanl to %s", c.tickInterval))
	} else {
		// if we got here, either we have no resources, or all resources are busy.
		// In this state, it's possible that the interval is less than 1 second,
		// because we previously set it to a small value for an immediate refresh.
		// in this case, we want to reset it to a sane value
		if c.tickInterval < time.Second {
			c.SetTickInterval(minInterval)
			c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: resetting check intervanl to %s after forced refresh", c.tickInterval))
		}
	}

	c.traceSink.Put(ctx, fmt.Sprintf("httprc controller: next check in %s", c.tickInterval))
}

func (c *ctrlBackend) SetTickInterval(d time.Duration) {
	// TODO synchronize
	if d <= 0 {
		d = time.Second // ensure positive interval
	}
	c.tickInterval = d
	c.check.Reset(d)
}
