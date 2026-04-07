package ai

import (
	"context"
	"coolaid/internal/llm"
	"coolaid/internal/prompts"
	"coolaid/internal/vector"
)

// ChatSession represents a stateful conversation with the LLM.
//
// It maintains a history of user and assistant messages and
// performs retrieval-augmented generation (RAG) on each turn.
type ChatSession struct {
	client  *Client
	history []llm.Message

	cfg *taskConfig
}

// NewChatSession creates a new stateful chat session.
//
// A ChatSession manages conversation history across multiple turns
// and enables retrieval-augmented responses using the codebase index.
//
// Unlike one-shot methods (e.g. Ask or Query), a ChatSession:
// - maintains user and assistant messages in memory
// - injects retrieval context on a per-message basis (not persisted)
// - uses the LLM chat endpoint to support multi-turn interactions
//
// The returned session is not thread-safe and should be used
// by a single caller.
func (c *Client) NewChatSession(opts ...TaskOption) *ChatSession {
	return &ChatSession{
		client: c,
		cfg:    parseTaskOptions(opts...),
	}
}

// Send sends a user message to the chat session and streams the response.
//
// It:
// - appends the raw user message to history
// - performs semantic search using the latest message
// - injects retrieved context into a temporary prompt (not persisted)
// - streams the LLM response
// - appends the assistant response to history
//
// Retrieval is best-effort: if no relevant context is found, the model
// responds using conversation history alone.
func (s *ChatSession) Send(ctx context.Context, msg string) error {
	// append original user message
	s.history = append(s.history, userMsg(msg))

	// try retrieval (non-blocking)
	results, err := s.client.SemanticSearch(ctx, msg, s.cfg.retrieval.k, s.cfg.retrieval.useMMR)
	if err != nil {
		return err
	}

	// build prompt (with or without results)
	prompt, err := prompts.Render(&prompts.Config{
		Template:       prompts.TemplateChat,
		SystemOverride: s.cfg.prompt.systemOverride,
		Structured:     s.cfg.prompt.structuredOutput,
	}, msg, vector.ToContextChunks(results...)...)
	if err != nil {
		return err
	}

	resp, err := s.client.llm.ChatStream(append(s.history[:len(s.history)-1], userMsg(prompt)), s.client.writer)
	if err != nil {
		return err
	}

	// append assistant response
	s.history = append(s.history, assistantMsg(resp))
	return nil
}

// Reset clears the current history to start a brand new chat.
func (s *ChatSession) Reset() {
	s.history = nil
}

// History returns the cached history.
func (s *ChatSession) History() []llm.Message {
	return s.history
}

func userMsg(msg string) llm.Message {
	return llm.Message{Role: llm.RoleUser, Content: msg}
}

func assistantMsg(msg string) llm.Message {
	return llm.Message{Role: llm.RoleAssistant, Content: msg}
}
