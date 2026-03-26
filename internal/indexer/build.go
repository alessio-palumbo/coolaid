package indexer

import (
	"ai-cli/internal/config"
	"ai-cli/internal/llm"
	"ai-cli/internal/vector"
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
	for _, file := range files {
		content, err := LoadFile(file)
		if err != nil {
			continue
		}

		summaryBuilder.AddFile(file, content)
		chunks := ChunkFile(file, content)
		for _, chunk := range chunks {
			embedding, err := client.Embed(chunk.Text)
			if err != nil {
				continue
			}
			store.Add(file, chunk.Text, chunk.StartLine, chunk.EndLine, embedding)
		}
	}

	store.AddSummary(summaryBuilder.Build())
	return nil
}
