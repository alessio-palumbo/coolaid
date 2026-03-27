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

func LoadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func isTextFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)

	return isLikelyText(buf[:n])
}
func isLikelyText(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return false // binary
		}
	}
	return true
}
