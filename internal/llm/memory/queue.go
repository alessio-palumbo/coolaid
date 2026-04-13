package memory

import (
	"context"
	"sync"
	"time"
)

const (
	memoryQueueBuffer = 50
	workerTimeout     = 5 * time.Second
)

type queue struct {
	ch      chan Input
	service *service
	wg      sync.WaitGroup
}

func newQueue(s *service) *queue {
	q := &queue{
		ch:      make(chan Input, memoryQueueBuffer),
		service: s,
	}

	q.wg.Go(q.loop)
	return q
}

func (q *queue) close(ctx context.Context) error {
	close(q.ch)

	done := make(chan struct{})

	go func() {
		q.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *queue) Enqueue(in Input) {
	select {
	case q.ch <- in:
	default:
	}
}

func (q *queue) loop() {
	for in := range q.ch {
		ctx, cancel := context.WithTimeout(context.Background(), workerTimeout)
		_ = q.service.extractAndUpdate(ctx, in) // best-effort
		cancel()
	}
}
