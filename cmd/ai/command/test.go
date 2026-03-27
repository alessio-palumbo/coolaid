package command

import (
	"ai-cli/internal/llm"
	"ai-cli/internal/prompts"
	"ai-cli/internal/query"
	"ai-cli/internal/vector"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
)

func TestCommand(llmClient *llm.Client, store *vector.Store) *cli.Command {
	return &cli.Command{
		Name:  "test",
		Usage: "generate tests for a file or function",
		Action: func(c *cli.Context) error {
			arg := c.Args().First()
			if arg == "" {
				return fmt.Errorf("target required")
			}

			file, fn := parseTarget(arg)
			supportedFn := isSupportedLanguage(file)
			if !supportedFn {
				slog.Info("⚠️ Function-level targeting for  is best-effort (no AST support)")
			}

			content, err := os.ReadFile(file)
			if err != nil {
				return err
			}

			signals := query.ExtractSignals(file, content)
			results, err := embedPromptAndSearch(llmClient, store, signals, vector.SearchModeDeep)
			if err != nil {
				return err
			}

			var prompt string
			if supportedFn {
				targetCode := extractTarget(content, fn)
				prompt, err = prompts.Render(
					&prompts.Config{Template: prompts.TemplateTestGo},
					targetCode,
					results...,
				)
			} else {
				targetCode := extractTarget(content, fn)
				prompt, err = prompts.Render(
					&prompts.Config{Template: prompts.TemplateTestGeneric},
					targetCode,
					results...,
				)
			}
			if err != nil {
				return err
			}
			return llmClient.GenerateStream(prompt, os.Stdout)
		},
	}
}

// isSupportedLanguage returns true if the test are supported for the given extension.
func isSupportedLanguage(path string) bool {
	switch filepath.Ext(path) {
	case ".go":
		return true
	}
	return false
}

// parseTarget splits input like "file.go:FuncName" into file path and function name.
// If no function is specified, fn will be empty.
func parseTarget(arg string) (file string, fn string) {
	parts := strings.SplitN(arg, ":", 2)
	file = parts[0]

	if len(parts) == 2 {
		fn = strings.TrimSpace(parts[1])
	}
	return file, fn
}

// extractTarget returns either the full file content or a specific function body.
// If fn is empty, the full content is returned.
// If fn is not found, it falls back to full content.
func extractTarget(src []byte, fn string) string {
	if fn == "" {
		return string(src)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return string(src) // fallback
	}

	for _, decl := range file.Decls {
		f, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		if f.Name.Name != fn {
			continue
		}

		start := fset.Position(f.Pos()).Offset
		end := fset.Position(f.End()).Offset
		return string(src[start:end])
	}

	// fallback if function not found
	return string(src)
}

func extractFunctionByNameFallback(src, fn string) string {
	lines := strings.Split(src, "\n")

	var out []string
	capture := false

	for _, line := range lines {
		if strings.Contains(line, fn) {
			capture = true
		}
		if capture {
			out = append(out, line)
		}
		if capture && strings.TrimSpace(line) == "" {
			break
		}
	}

	if len(out) > 0 {
		return strings.Join(out, "\n")
	}
	return src
}

func targetLabel(file, fn string) string {
	if fn == "" {
		return file
	}
	return fmt.Sprintf("%s:%s", file, fn)
}
