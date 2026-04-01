package indexer

import (
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

func Build(client *llm.Client, store *vector.Store, ignorePatterns []string, extensions map[string]struct{}, onProgress ProgressFunc) error {
	ignore, err := LoadIgnore(store.ProjectRoot, ignorePatterns)
	if err != nil {
		return err
	}

	files, err := Scan(store.ProjectRoot, ignore, extensions)
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
