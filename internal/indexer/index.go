package indexer

import (
	"ai-cli/internal/llm"
	"ai-cli/internal/vector"
)

const defaultChunkCharacters = 400

func Build(dir string, client *llm.Client) (*vector.Store, error) {
	files, err := Scan(dir)
	if err != nil {
		return nil, err
	}

	store := &vector.Store{}

	for _, file := range files {
		content, err := LoadFile(file)
		if err != nil {
			continue
		}

		chunks := Chunk(content, defaultChunkCharacters)
		for _, chunk := range chunks {
			embedding, err := client.Embed(chunk)
			if err != nil {
				continue
			}
			store.Add(chunk, embedding)
		}
	}

	return store, nil
}
