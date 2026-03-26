package indexer

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

var defaultIgnorePatterns = []string{
	".git/",
	"node_modules/",
	"vendor/",
	"dist/",
	"build/",
	"*.lock",
	"*.min.js",
}

type pattern struct {
	clean    string
	isDir    bool
	hasSlash bool
	isSimple bool
}

type Ignore struct {
	segmentSimple []pattern
	segmentGlob   []pattern
	pathPatterns  []pattern
}

func LoadIgnore(projectRoot string, userIgnorePatterns []string) (*Ignore, error) {
	patterns := append(defaultIgnorePatterns, userIgnorePatterns...)

	files := []string{
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

	return NewIgnoreFromPatterns(patterns...), nil
}

// NewIgnoreFromPatterns  precomputes patterns for efficient matching.
func NewIgnoreFromPatterns(patterns ...string) *Ignore {
	var ig Ignore
	for _, p := range patterns {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p == "" {
			continue
		}

		isDir := strings.HasSuffix(p, "/")
		clean := strings.TrimSuffix(p, "/")
		hasSlash := strings.Contains(clean, "/")
		isSimple := !strings.ContainsAny(clean, "*?[]")

		pt := pattern{
			clean:    clean,
			isDir:    isDir,
			hasSlash: hasSlash,
			isSimple: isSimple,
		}

		if !hasSlash {
			if isSimple {
				ig.segmentSimple = append(ig.segmentSimple, pt)
			} else {
				ig.segmentGlob = append(ig.segmentGlob, pt)
			}
		} else {
			ig.pathPatterns = append(ig.pathPatterns, pt)
		}
	}

	return &ig
}

func (i *Ignore) Match(path string) bool {
	path = filepath.ToSlash(path)
	base := filepath.Base(path)

	for _, p := range i.segmentSimple {
		for seg := range strings.SplitSeq(path, "/") {
			if seg == p.clean {
				return true
			}
		}
	}

	for _, p := range i.segmentGlob {
		for seg := range strings.SplitSeq(path, "/") {
			if match, _ := filepath.Match(p.clean, seg); match {
				return true
			}
		}
	}

	for _, p := range i.pathPatterns {
		// fast prefix reject for dir patterns
		if p.isDir && !strings.HasPrefix(path, p.clean) {
			continue
		}

		if match, _ := filepath.Match(p.clean, path); match {
			return true
		}

		if !p.isDir {
			if match, _ := filepath.Match(p.clean, base); match {
				return true
			}
		}
	}

	return false
}
