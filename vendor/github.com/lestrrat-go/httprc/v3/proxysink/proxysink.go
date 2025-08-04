package proxysink

import (
	"context"
	"sync"
)

type Backend[T any] interface {
	Put(context.Context, T)
}

// Proxy is used to send values through a channel. This is used to
// serialize calls to underlying sinks.
type Proxy[T any] struct {
	mu      *sync.Mutex
	cancel  context.CancelFunc
	ch      chan T
	cond    *sync.Cond
	pending []T
	backend Backend[T]
	closed  bool
}

func New[T any](b Backend[T]) *Proxy[T] {
	mu := &sync.Mutex{}
	return &Proxy[T]{
		ch:      make(chan T, 1),
		mu:      mu,
		cond:    sync.NewCond(mu),
		backend: b,
		cancel:  func() {},
	}
}

func (p *Proxy[T]) Run(ctx context.Context) {
	defer p.cond.Broadcast()

	p.mu.Lock()
	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.mu.Unlock()

	go p.controlloop(ctx)
	go p.flushloop(ctx)

	<-ctx.Done()
}

func (p *Proxy[T]) controlloop(ctx context.Context) {
	defer p.cond.Broadcast()
	for {
		select {
		case <-ctx.Done():
			return
		case r := <-p.ch:
			p.mu.Lock()
			p.pending = append(p.pending, r)
			p.mu.Unlock()
		}
		p.cond.Broadcast()
	}
}

func (p *Proxy[T]) flushloop(ctx context.Context) {
	const defaultPendingSize = 10
	pending := make([]T, defaultPendingSize)
	for {
		select {
		case <-ctx.Done():
			p.mu.Lock()
			if len(p.pending) <= 0 {
				p.mu.Unlock()
				return
			}
		default:
		}

		p.mu.Lock()
		for len(p.pending) <= 0 {
			select {
			case <-ctx.Done():
				p.mu.Unlock()
				return
			default:
				p.cond.Wait()
			}
		}

		// extract all pending values, and clear the shared slice
		if cap(pending) < len(p.pending) {
			pending = make([]T, len(p.pending))
		} else {
			pending = pending[:len(p.pending)]
		}
		copy(pending, p.pending)
		if cap(p.pending) > defaultPendingSize {
			p.pending = make([]T, 0, defaultPendingSize)
		} else {
			p.pending = p.pending[:0]
		}
		p.mu.Unlock()

		for _, v := range pending {
			// send to sink serially
			p.backend.Put(ctx, v)
		}
	}
}

func (p *Proxy[T]) Put(ctx context.Context, v T) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	select {
	case <-ctx.Done():
		return
	case p.ch <- v:
		return
	}
}

func (p *Proxy[T]) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.closed {
		p.closed = true
	}
	p.cancel()
	p.cond.Broadcast()
}
