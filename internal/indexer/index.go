package indexer

import (
	"ai-cli/internal/llm"
	"ai-cli/internal/vector"
)

func Build(dir string, store *vector.Store, client *llm.Client, configDir string) error {
	ignore, err := LoadIgnore(store.ProjectRoot, configDir)
	if err != nil {
		return err
	}

	files, err := Scan(dir, ignore)
	if err != nil {
		return err
	}

	for _, file := range files {
		content, err := LoadFile(file)
		if err != nil {
			continue
		}

		chunks := ChunkFile(file, content)
		for _, chunk := range chunks {
			embedding, err := client.Embed(chunk.Text)
			if err != nil {
				continue
			}
			store.Add(file, chunk.Text, chunk.StartLine, chunk.EndLine, embedding)
		}
	}

	return nil
}
