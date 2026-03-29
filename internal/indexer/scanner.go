package indexer

import (
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
)

const maxFileSize = 5 * 1024 * 1024 // 5MB

var specialFiles = map[string]struct{}{
	"Dockerfile": {},
	"Makefile":   {},
}

func Scan(dir string, ignore *Ignore, exts map[string]struct{}) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Debug("filepath error, skipping", slog.String("error", err.Error()))
			return nil
		}

		// Skip files that are too big and might block the pipeline.
		info, err := d.Info()
		if err == nil && info.Size() > maxFileSize {
			return nil
		}

		// Use relative path for matching as ignore rules are project-relative.
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		isDir := d.IsDir()
		if ignore.Match(relPath, isDir) {
			if isDir {
				// Skip entire subtree
				return filepath.SkipDir
			}
			return nil
		}

		if _, ok := specialFiles[filepath.Base(path)]; ok {
			files = append(files, path)
			return nil
		}
		if _, ok := exts[strings.ToLower(filepath.Ext(path))]; ok {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}
