package indexer

import (
	"ai-cli/internal/config"
	"ai-cli/internal/llm"
	"ai-cli/internal/vector"
	"os"
)

func Build(dir string, store *vector.Store, client *llm.Client, cfg *config.Config) error {
	ignore, err := LoadIgnore(store.ProjectRoot, cfg.Index.IgnorePatterns)
	if err != nil {
		return err
	}

	files, err := Scan(store.ProjectRoot, ignore, cfg.Extensions)
	if err != nil {
		return err
	}

	summaryBuilder := NewSummaryBuilder()
	pipeline := NewEmbedPipeline(client, store, len(files))

	for _, file := range files {
		content, err := LoadFile(file)
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

func LoadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return data, nil
}
