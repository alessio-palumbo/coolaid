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
	TemplateExplain     promptTemplate = "explain.tmpl"
	TemplateSummarize   promptTemplate = "summarize.tmpl"
	TemplateQuery       promptTemplate = "query.tmpl"
	TemplateChat        promptTemplate = "chat.tmpl"
	TemplateTestGo      promptTemplate = "test-go.tmpl"
	TemplateTestGeneric promptTemplate = "test-generic.tmpl"
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
	Summary    string
}

type templateData struct {
	Formatting string
	Prompt     string
	Context    string
	Summary    string
}

func Render(cfg *Config, prompt string, context ...vector.Result) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("template config must be set")
	}

	if cfg.Structured {
		prompt += structuredInstructions
	}
	td := templateData{
		Formatting: formattingDirectives,
		Prompt:     prompt,
		Context:    vector.JoinResults(context...),
		Summary:    cfg.Summary,
	}

	var buf bytes.Buffer
	if err := templates.Lookup(string(cfg.Template)).
		Execute(&buf, td); err != nil {
		return "", err
	}
	return buf.String(), nil
}
