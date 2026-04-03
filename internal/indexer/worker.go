package indexer

import (
	"ai-cli/internal/llm"
	"ai-cli/internal/vector"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
)

type EmbedPipeline struct {
	jobs    chan embedJob
	results chan embedResult

	client *llm.Client
	store  *vector.Store
	logger *slog.Logger

	wg sync.WaitGroup

	total int64
	done  atomic.Int64

	onProgress ProgressFunc
}

type embedJob struct {
	file    string
	content []byte
}

type embedResult struct {
	file      string
	chunk     Chunk
	embedding []float64
	err       error
}

func NewEmbedPipeline(client *llm.Client, store *vector.Store, logger *slog.Logger, maxWorkers, totalFiles int, onProgress ProgressFunc) *EmbedPipeline {
	p := &EmbedPipeline{
		jobs:       make(chan embedJob, 100),
		results:    make(chan embedResult, 100),
		client:     client,
		store:      store,
		logger:     logger,
		total:      int64(totalFiles),
		onProgress: onProgress,
	}

	for range workerPool(maxWorkers) {
		p.wg.Go(p.worker)
	}

	go p.collector()
	return p
}

func (p *EmbedPipeline) Submit(job embedJob) {
	p.jobs <- job
}

func (p *EmbedPipeline) Wait() {
	close(p.jobs)
	p.wg.Wait()
	close(p.results)
}

func (p *EmbedPipeline) worker() {
	for job := range p.jobs {
		chunks := ChunkFile(job.file, job.content)
		for _, chunk := range chunks {
			embedding, err := p.client.Embed(chunk.Text)
			p.results <- embedResult{
				file:      job.file,
				embedding: embedding,
				chunk:     chunk,
				err:       err,
			}
		}
	}
}

func (p *EmbedPipeline) collector() {
	filesDone := make(map[string]struct{})
	for res := range p.results {
		if res.err != nil {
			p.logger.Warn(
				"embed error",
				slog.String("error", res.err.Error()),
				slog.String("file", res.file),
				slog.Int("start_line", res.chunk.StartLine),
				slog.Int("end_line", res.chunk.EndLine),
			)
			continue
		}

		p.store.AddItem(vector.Item{
			FilePath:  res.file,
			Symbol:    res.chunk.Symbol,
			Kind:      res.chunk.Kind,
			StartLine: res.chunk.StartLine,
			EndLine:   res.chunk.EndLine,
			Content:   res.chunk.Text,
			Embedding: res.embedding,
		})

		// increment done per file
		if _, ok := filesDone[res.file]; !ok {
			filesDone[res.file] = struct{}{}
			p.done.Add(1)

			var size int64
			if info, err := os.Stat(res.file); err == nil {
				size = info.Size()
			}

			if p.onProgress != nil {
				p.onProgress(Progress{
					Done:  p.done.Load(),
					Total: p.total,
					File:  res.file,
					Size:  size,
				})
			}
		}
	}
}

func workerPool(maxWorkers int) int {
	if maxWorkers < 1 {
		n := runtime.NumCPU()

		switch {
		case n <= 4:
			return n
		case n <= 8:
			return 4
		default:
			return 6
		}
	}
	return maxWorkers
}
