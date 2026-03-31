package indexer

import (
	"ai-cli/internal/llm"
	"ai-cli/internal/vector"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const (
	pipelineWorkes          = 4
	progressRefreshDuration = 500 * time.Millisecond
)

type EmbedPipeline struct {
	jobs    chan embedJob
	results chan embedResult

	client *llm.Client
	store  *vector.Store

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
	embedding []float64
	chunkText string
	startLine int
	endLine   int
	err       error
}

func NewEmbedPipeline(client *llm.Client, store *vector.Store, totalFiles int, onProgress ProgressFunc) *EmbedPipeline {
	p := &EmbedPipeline{
		jobs:       make(chan embedJob, 100),
		results:    make(chan embedResult, 100),
		client:     client,
		store:      store,
		total:      int64(totalFiles),
		onProgress: onProgress,
	}

	for range pipelineWorkes {
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
				chunkText: chunk.Text,
				startLine: chunk.StartLine,
				endLine:   chunk.EndLine,
				err:       err,
			}
		}
	}
}

func (p *EmbedPipeline) collector() {
	filesDone := make(map[string]struct{})
	for res := range p.results {
		if res.err != nil {
			slog.Warn(
				"embed error",
				slog.String("error", res.err.Error()),
				slog.String("file", res.file),
				slog.Int("start_line", res.startLine),
				slog.Int("end_line", res.endLine),
			)
			continue
		}

		p.store.Add(res.file, res.chunkText, res.startLine, res.endLine, res.embedding)

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
