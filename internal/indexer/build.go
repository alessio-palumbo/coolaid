package indexer

import (
	"context"
	"coolaid/internal/llm"
	"coolaid/internal/store"
	"log/slog"
	"os"
)

// IndexOptions defines how the indexing process is performed.
//
// It controls which files are included, which are ignored, and how
// the indexing workload is executed.
type IndexOptions struct {
	// IgnorePatterns defines glob patterns for files or directories
	// to exclude from indexing (e.g. "vendor/**", "node_modules/**").
	IgnorePatterns []string

	// Extensions is the set of allowed file extensions to index.
	// It should contain normalized extensions (lowercased, with a leading dot).
	// Files with extensions not in this set will be skipped.
	Extensions map[string]struct{}

	// MaxWorkers controls the maximum number of concurrent workers
	// used during indexing.
	//
	// If set to less than 1, a conservative default is used to avoid
	// overwhelming external systems (e.g. embedding/LLM services).
	MaxWorkers int
}

// Progress represents the indexing progress at a point in time.
//
// It is emitted via ProgressFunc as files are processed.
type Progress struct {
	// Done is the number of files processed so far.
	Done int64

	// Total is the total number of files to process.
	Total int64

	// File is the current file being processed.
	File string

	// Size is the size of the current file in bytes.
	Size int64
}

// ProgressFunc is a callback used to report indexing progress.
//
// It is invoked as files are processed. The caller is responsible
// for handling how progress is displayed or consumed.
type ProgressFunc func(Progress)

// Build scans the given project, generates embeddings for supported files,
// and stores the results in the provided vector store.
//
// It processes files according to the provided IndexOptions and reports
// progress via the onProgress callback (if provided).
//
// Build is intended to be used internally by higher-level APIs and does not
// perform any output or rendering itself.
func Build(ctx context.Context, client *llm.Client, store *store.Store, logger *slog.Logger, opts IndexOptions, onProgress ProgressFunc) error {
	ignore, err := LoadIgnore(store.ProjectRoot, opts.IgnorePatterns)
	if err != nil {
		return err
	}

	files, err := Scan(store.ProjectRoot, ignore, opts.Extensions)
	if err != nil {
		return err
	}

	summaryBuilder := NewSummaryBuilder()
	pipeline := NewEmbedPipeline(ctx, client, store, logger, opts.MaxWorkers, len(files), onProgress)

	for _, file := range files {
		content, err := loadFile(file)
		if err != nil {
			continue
		}

		summaryBuilder.AddFile(file, content)
		pipeline.Submit(embedJob{file: file, content: content})
	}

	pipeline.Wait()
	store.AddSummary(summaryBuilder.Build())
	return nil
}

func loadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return data, nil
}
