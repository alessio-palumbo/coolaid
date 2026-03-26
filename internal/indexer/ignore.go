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
	path = filepath.ToSlash(path)
	base := filepath.Base(path)

	for _, pattern := range i.patterns {
		pattern = filepath.ToSlash(pattern)

		isDirPattern := strings.HasSuffix(pattern, "/")
		cleanPattern := strings.TrimSuffix(pattern, "/")

		// CASE 1: pattern has no slash → match any path segment
		if !strings.Contains(cleanPattern, "/") {
			for seg := range strings.SplitSeq(path, "/") {
				match, err := filepath.Match(cleanPattern, seg)
				if err == nil && match {
					// if directory-only pattern, ensure it's not matching a file
					if isDirPattern {
						// assume directory if not last segment or explicitly known
						if seg != base {
							return true
						}
						continue
					}
					return true
				}
			}
			continue
		}

		// CASE 2: full path match
		match, err := filepath.Match(cleanPattern, path)
		if err == nil && match {
			if isDirPattern {
				// must match a directory prefix
				if strings.HasPrefix(path, cleanPattern+"/") || path == cleanPattern {
					return true
				}
				continue
			}
			return true
		}

		// CASE 3: basename fallback
		match, err = filepath.Match(cleanPattern, base)
		if err == nil && match && !isDirPattern {
			return true
		}
	}

	return false
}
