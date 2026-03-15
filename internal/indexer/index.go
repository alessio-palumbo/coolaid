package indexer

import (
	"ai-cli/internal/llm"
	"ai-cli/internal/vector"
)

func Build(dir string, client *llm.Client) (*vector.Store, error) {
	ignore, err := LoadIgnore()
	if err != nil {
		return nil, err
	}

	files, err := Scan(dir, ignore)
	if err != nil {
		return nil, err
	}

	store := &vector.Store{}

	for _, file := range files {
		content, err := LoadFile(file)
		if err != nil {
			continue
		}

		chunks := ChunkFile(file, content)
		for _, chunk := range chunks {
			embedding, err := client.Embed(chunk)
			if err != nil {
				continue
			}
			store.Add(chunk, file, embedding)
		}
	}

	return store, nil
}
