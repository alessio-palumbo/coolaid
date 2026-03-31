package indexer

import (
	"ai-cli/internal/config"
	"ai-cli/internal/llm"
	"ai-cli/internal/vector"
	"os"
)

type Progress struct {
	Done  int64
	Total int64
	File  string
	Size  int64
}

type ProgressFunc func(Progress)

func Build(cfg *config.Config, client *llm.Client, store *vector.Store, onProgress ProgressFunc) error {
	ignore, err := LoadIgnore(store.ProjectRoot, cfg.Index.IgnorePatterns)
	if err != nil {
		return err
	}

	files, err := Scan(store.ProjectRoot, ignore, cfg.Extensions)
	if err != nil {
		return err
	}

	summaryBuilder := NewSummaryBuilder()
	pipeline := NewEmbedPipeline(client, store, len(files), onProgress)

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
