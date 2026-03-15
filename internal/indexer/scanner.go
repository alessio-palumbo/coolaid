package indexer

import (
	"io/fs"
	"os"
	"path/filepath"
)

func Scan(dir string, ignore *Ignore) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() || ignore.Match(path) {
			return nil
		}

		switch filepath.Ext(path) {
		case ".go", ".py", ".ts", ".js", ".md":
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
