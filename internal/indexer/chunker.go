package indexer

func Chunk(text string, size int) []string {
	var chunks []string
	runes := []rune(text)

	for i := 0; i < len(runes); i += size {
		end := min(i+size, len(runes))
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}
