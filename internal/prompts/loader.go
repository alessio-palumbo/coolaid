package prompts

import (
	"ai-cli/internal/vector"
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

type promptTemplate string

const (
	TemplateExplain   promptTemplate = "explain.tmpl"
	TemplateSummarize promptTemplate = "summarize.tmpl"
	TemplateQuery     promptTemplate = "query.tmpl"
)

type promptMode int

const (
	PromptSimple promptMode = iota
	PromptStructured
)

const structuredInstructions = `
Structure your answer as:
1. Summary
2. Key components involved
3. Explanation
`

const formattingDirectives = `
Use clear formatting:
- Short paragraphs
- Bullet points for lists
- Code formatting for commands or identifiers
`

//go:embed templates/*.tmpl
var promptFS embed.FS

var templates = template.Must(template.ParseFS(promptFS, "templates/*.tmpl"))

type Config struct {
	Template   promptTemplate
	Structured bool
}

type templateData struct {
	Formatting string
	Prompt     string
	Context    string
}

func Render(cfg *Config, prompt string, context ...vector.Result) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("template config must be set")
	}

	if cfg.Structured {
		prompt += structuredInstructions
	}
	td := templateData{Formatting: formattingDirectives, Prompt: prompt}
	for _, r := range context {
		td.Context += fmt.Sprintf(
			"File: %s (lines %d-%d)\n%s\n---\n",
			r.FilePath,
			r.StartLine,
			r.EndLine,
			r.Content,
		)
	}

	var buf bytes.Buffer
	if err := templates.Lookup(string(cfg.Template)).
		Execute(&buf, td); err != nil {
		return "", err
	}
	return buf.String(), nil
}
