package httprc

import (
	"context"
	"fmt"
	"sync"
)

type worker struct {
	httpcl    HTTPClient
	incoming  chan any
	next      <-chan Resource
	nextsync  <-chan synchronousRequest
	errSink   ErrorSink
	traceSink TraceSink
}

func (w worker) Run(ctx context.Context, readywg *sync.WaitGroup, donewg *sync.WaitGroup) {
	w.traceSink.Put(ctx, "httprc worker: START worker loop")
	defer w.traceSink.Put(ctx, "httprc worker: END   worker loop")
	defer donewg.Done()
	ctx = withTraceSink(ctx, w.traceSink)
	ctx = withHTTPClient(ctx, w.httpcl)

	readywg.Done()
	for {
		select {
		case <-ctx.Done():
			w.traceSink.Put(ctx, "httprc worker: stopping worker loop")
			return
		case r := <-w.next:
			w.traceSink.Put(ctx, fmt.Sprintf("httprc worker: syncing %q (async)", r.URL()))
			if err := r.Sync(ctx); err != nil {
				w.errSink.Put(ctx, err)
			}
			r.SetBusy(false)

			w.sendAdjustIntervalRequest(ctx, r)
		case sr := <-w.nextsync:
			w.traceSink.Put(ctx, fmt.Sprintf("httprc worker: syncing %q (synchronous)", sr.resource.URL()))
			if err := sr.resource.Sync(ctx); err != nil {
				sendReply(ctx, sr.reply, struct{}{}, err)
				sr.resource.SetBusy(false)
				return
			}
			sr.resource.SetBusy(false)
			sendReply(ctx, sr.reply, struct{}{}, nil)
			w.sendAdjustIntervalRequest(ctx, sr.resource)
		}
	}
}

func (w worker) sendAdjustIntervalRequest(ctx context.Context, r Resource) {
	w.traceSink.Put(ctx, "httprc worker: Sending interval adjustment request for "+r.URL())
	select {
	case <-ctx.Done():
	case w.incoming <- adjustIntervalRequest{resource: r}:
	}
	w.traceSink.Put(ctx, "httprc worker: Sent interval adjustment request for "+r.URL())
}
