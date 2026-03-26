package indexer

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var specialFiles = map[string]struct{}{
	"Dockerfile": {},
	"Makefile":   {},
}

func Scan(dir string, ignore *Ignore, exts map[string]struct{}) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if ignore.Match(path) {
			if d.IsDir() {
				// Skip entire subtree
				return filepath.SkipDir
			}
			return nil
		}

		if _, ok := specialFiles[filepath.Base(path)]; ok {
			files = append(files, path)
		}
		if _, ok := exts[strings.ToLower(filepath.Ext(path))]; ok {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

func LoadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
