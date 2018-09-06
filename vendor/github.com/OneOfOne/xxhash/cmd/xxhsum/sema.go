package main

import "sync"

type sema struct {
	wg sync.WaitGroup
	ch chan func()
}

func newSema(n int) *sema {
	s := &sema{
		ch: make(chan func(), n),
	}
	for ; n > 0; n-- {
		go s.handler()
	}
	return s
}

func (s *sema) handler() {
	for fn := range s.ch {
		fn()
		s.wg.Done()
	}
}

func (s *sema) Run(fn func()) {
	s.wg.Add(1)
	s.ch <- fn
}

func (s *sema) WaitAndClose() {
	s.wg.Wait()
	close(s.ch)
	s.ch = nil
}
