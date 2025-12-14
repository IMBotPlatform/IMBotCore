package ai

import (
	"context"

	"github.com/tmc/langchaingo/llms"
)

// SessionStore manages the persistence of chat history.
type SessionStore interface {
	// GetHistory retrieves the chat history for a given session.
	GetHistory(ctx context.Context, sessionID string) ([]llms.ChatMessage, error)

	// AddUserMessage adds a user message to the session history.
	AddUserMessage(ctx context.Context, sessionID, text string) error

	// AddAIMessage adds an AI response to the session history.
	AddAIMessage(ctx context.Context, sessionID, text string) error

	// ClearHistory clears the session history.
	ClearHistory(ctx context.Context, sessionID string) error
}
