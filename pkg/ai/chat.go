package ai

import (
	"context"
	"coolaid/internal/core/engine"
	"coolaid/internal/llm"
)

// ChatSession represents a stateful conversation with the LLM.
//
// It maintains a history of user and assistant messages and
// performs retrieval-augmented generation (RAG) on each turn.
type ChatSession struct {
	client  *Client
	history []llm.Message

	cfg *engine.TaskConfig
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
// - performs semantic search using the current message
// - injects retrieved context into a temporary prompt (not persisted)
// - streams the LLM response
// - appends the raw user message to history
// - appends the assistant response to history
//
// Retrieval is best-effort: if no relevant context is found, the model
// responds using conversation history alone.
func (s *ChatSession) Send(ctx context.Context, msg string) error {
	resp, err := s.client.engine.RunChat(ctx, engine.ChatRequest{
		Msg:     msg,
		History: s.history,
		Config:  s.cfg,
	})
	if err != nil {
		return err
	}

	s.history = append(
		s.history,
		llm.Message{Role: llm.RoleUser, Content: msg},
		llm.Message{Role: llm.RoleAssistant, Content: resp},
	)
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
