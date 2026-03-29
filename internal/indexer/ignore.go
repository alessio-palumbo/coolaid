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
	clean      string
	isDir      bool
	isCompound bool
	isSimple   bool
	negated    bool
}

type Ignore struct {
	patterns []pattern
}

// LoadIgnore builds an Ignore matcher by combining default patterns,
// user-provided patterns, and ignore files found in the project root.
//
// Patterns are loaded in the following order (lowest to highest precedence):
//  1. defaultIgnorePatterns
//  2. userIgnorePatterns
//  3. .gitignore
//  4. .aiignore
//
// Lines that are empty or start with '#' are ignored.
// Patterns are passed as-is to NewIgnoreFromPatterns and evaluated in order,
// where the last matching rule wins (including support for '!' negation).
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

// NewIgnoreFromPatterns precomputes patterns for efficient matching.
func NewIgnoreFromPatterns(patterns ...string) *Ignore {
	var ig Ignore
	for _, p := range patterns {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p == "" {
			continue
		}

		negated := strings.HasPrefix(p, "!")
		if negated {
			p = strings.TrimPrefix(p, "!")
		}

		isDir := strings.HasSuffix(p, "/")
		clean := strings.TrimSuffix(p, "/")
		isCompound := strings.Contains(clean, "/")
		isSimple := !strings.ContainsAny(clean, "*?[]")

		ig.patterns = append(ig.patterns, pattern{
			clean:      clean,
			isDir:      isDir,
			isCompound: isCompound,
			isSimple:   isSimple,
			negated:    negated,
		})
	}

	return &ig
}

// Match reports whether the given path should be ignored.
//
// Paths are matched using gitignore-like semantics:
//   - All patterns are evaluated in order
//   - If a pattern matches, it overrides the previous state
//   - Negated patterns ('!') re-include paths
//   - The final matching rule determines the result
//
// The path must be relative and use '/' as separator (automatically normalized).
// Match does not short-circuit and always evaluates all patterns to ensure
// correct handling of negations.
func (i *Ignore) Match(path string, isDir bool) bool {
	path = filepath.ToSlash(path)
	base := filepath.Base(path)

	ignored := false

	for _, p := range i.patterns {
		matched := false

		if !p.isCompound {
			for seg := range strings.SplitSeq(path, "/") {
				if p.isSimple {
					matched = seg == p.clean
				} else {
					if ok, _ := filepath.Match(p.clean, seg); ok {
						matched = true
					}
				}
				if matched {
					if p.clean == base && p.isDir != isDir {
						matched = false
					}
					break
				}
			}
		} else {
			// directory-specific match, exact dir or anything under it
			if p.isDir && isDir {
				if path == p.clean || strings.HasPrefix(path, p.clean+"/") {
					matched = true
				}
			}

			// full path match (applies to all patterns)
			if !matched {
				if ok, _ := filepath.Match(p.clean, path); ok {
					matched = true
				}
			}

			// basename match (files only)
			if !matched && !p.isDir {
				if ok, _ := filepath.Match(p.clean, base); ok {
					matched = true
				}
			}
		}

		if matched {
			if p.negated {
				ignored = false
			} else {
				ignored = true
			}
		}
	}

	return ignored
}
