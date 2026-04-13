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

func (q *queue) close() {
	close(q.ch)
	q.wg.Wait()
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
