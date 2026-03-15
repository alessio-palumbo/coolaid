package indexer

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type Ignore struct {
	patterns []string
}

func LoadIgnore() (*Ignore, error) {
	file, err := os.Open(".aiignore")
	if err != nil {
		if os.IsNotExist(err) {
			return &Ignore{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var patterns []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	return &Ignore{patterns: patterns}, nil
}

func (i *Ignore) Match(path string) bool {
	for _, pattern := range i.patterns {
		match, _ := filepath.Match(pattern, filepath.Base(path))
		if match || strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}
