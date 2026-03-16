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

func LoadIgnore(projectRoot, configDir string) (*Ignore, error) {
	var patterns []string
	files := []string{
		filepath.Join(configDir, "ignore"), // global
		filepath.Join(projectRoot, ".gitignore"),
		filepath.Join(projectRoot, ".aiignore"),
	}

	for _, f := range files {
		file, err := os.Open(f)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {

			line := strings.TrimSpace(scanner.Text())

			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			patterns = append(patterns, line)
		}

		file.Close()
	}

	return &Ignore{patterns: patterns}, nil
}

func (i *Ignore) Match(path string) bool {
	for _, pattern := range i.patterns {
		// directory rule
		if strings.HasSuffix(pattern, "/") {
			dir := strings.TrimSuffix(pattern, "/")
			if strings.HasPrefix(path, dir+"/") {
				return true
			}
		}

		// full path match
		match, err := filepath.Match(pattern, path)
		if err == nil && match {
			return true
		}

		// basename match
		match, err = filepath.Match(pattern, filepath.Base(path))
		if err == nil && match {
			return true
		}
	}
	return false
}
