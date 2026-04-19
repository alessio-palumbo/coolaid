package prompts

import (
	"bytes"
	"coolaid/internal/retrieval"
	"coolaid/internal/store"
	"embed"
	"fmt"
	"text/template"
)

type Template string

const (
	TemplateExplain     Template = "explain.tmpl"
	TemplateSummarize   Template = "summarize.tmpl"
	TemplateQuery       Template = "query.tmpl"
	TemplateChat        Template = "chat.tmpl"
	TemplateTestGo      Template = "test-go.tmpl"
	TemplateTestGeneric Template = "test-generic.tmpl"
	TemplateAsk         Template = "ask.tmpl"
	TemplateEdit        Template = "edit.tmpl"
	TemplateFix         Template = "fix.tmpl"
	TemplateRefactor    Template = "refactor.tmpl"
	TemplateComment     Template = "comment.tmpl"
)

type targetType string

const (
	TargetFile     = "file"
	TargetFunction = "function"
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

// Precompiled templates
var templates = make(map[string]*template.Template)

func init() {
	var funcMap = template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}

	base := template.Must(template.New("base.tmpl").Funcs(funcMap).ParseFS(promptFS, "templates/base.tmpl"))

	// List all templates here (your "modes")
	files := []string{
		string(TemplateExplain),
		string(TemplateSummarize),
		string(TemplateQuery),
		string(TemplateChat),
		string(TemplateTestGo),
		string(TemplateTestGeneric),
		string(TemplateAsk),
		string(TemplateEdit),
		string(TemplateFix),
		string(TemplateRefactor),
		string(TemplateComment),
	}

	for _, file := range files {
		tmpl := template.Must(base.Clone())
		templates[file] = template.Must(tmpl.ParseFS(promptFS, "templates/"+file))
	}
}

type Config struct {
	Template       Template
	SystemOverride string
	Memory         store.Memory
	Structured     bool
	Summary        string
	Target         Target
}

func (c *Config) WithTarget(file, fn, body string) *Config {
	if fn != "" {
		c.Target = Target{
			Name: fn,
			Type: TargetFunction,
			Body: body,
		}
		return c
	}

	c.Target = Target{
		Name: file,
		Type: TargetFile,
		Body: body,
	}
	return c
}

type Target struct {
	Type targetType
	Name string
	Body string
}

type templateData struct {
	System                 string
	Memory                 store.Memory
	StructuredInstructions string
	Formatting             string
	Summary                string
	Target                 Target
	ContextChunks          []retrieval.Chunk
	Prompt                 string
}

func Render(cfg *Config, prompt string, context ...retrieval.Chunk) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("template config must be set")
	}

	tmpl, ok := templates[string(cfg.Template)]
	if !ok {
		return "", fmt.Errorf("template %s not found", cfg.Template)
	}

	td := templateData{
		System:        cfg.SystemOverride,
		Memory:        cfg.Memory,
		Formatting:    formattingDirectives,
		Summary:       cfg.Summary,
		Target:        cfg.Target,
		ContextChunks: context,
		Prompt:        prompt,
	}

	if cfg.Structured {
		td.StructuredInstructions = structuredInstructions
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base", td); err != nil {
		return "", err
	}
	return buf.String(), nil
}
