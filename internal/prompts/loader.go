package prompts

import (
	"bytes"
	"coolaid/internal/retrieval"
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
	TemplateAskWeb      promptTemplate = "ask-web.tmpl"
	TemplateEdit        promptTemplate = "edit.tmpl"
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
		string(TemplateAskWeb),
		string(TemplateEdit),
	}

	for _, file := range files {
		tmpl := template.Must(base.Clone())
		templates[file] = template.Must(tmpl.ParseFS(promptFS, "templates/"+file))
	}
}

type Config struct {
	Template       promptTemplate
	SystemOverride string
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
	StructuredInstructions string
	Formatting             string
	Prompt                 string
	ContextChunks          []retrieval.Chunk
	Summary                string
	Target                 Target
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
		Formatting:    formattingDirectives,
		Prompt:        prompt,
		ContextChunks: context,
		Summary:       cfg.Summary,
		Target:        cfg.Target,
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
