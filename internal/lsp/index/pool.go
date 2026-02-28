package index

import (
	"context"
	"runtime"

	"golang.org/x/sync/errgroup"
)

// NewPool returns a send-only channel for tasks and a wait function.
// When ctx is cancelled, the pool stops accepting tasks and wait returns ctx.Err().
// Caller sends func() error tasks, then closes the channel, then calls wait().
// Concurrency is limited to runtime.NumCPU()*2 when concurrency <= 0.
func NewPool(ctx context.Context, concurrency int) (chan<- func() error, func() error) {
	if concurrency <= 0 {
		concurrency = runtime.NumCPU() * 2
	}

	ch := make(chan func() error)
	done := make(chan error, 1)
	wait := func() error { return <-done }
	g, gctx := errgroup.WithContext(ctx)

	go func() {
		defer func() { done <- g.Wait() }()
		sem := make(chan struct{}, concurrency)
		for {
			select {
			case <-gctx.Done():
				return
			case task, ok := <-ch:
				if !ok {
					return
				}
				g.Go(func() error {
					select {
					case <-gctx.Done():
						return gctx.Err()
					case sem <- struct{}{}:
					}
					defer func() { <-sem }()
					return task()
				})
			}
		}
	}()

	return ch, wait
}
