// Package provider defines the LLM chat interface and official-API adapters.
package provider

import (
	"context"
	"fmt"
	"strings"

	"easy-cli/internal/config"
)

// Role values for Message.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Message is a chat message.
type Message struct {
	Role    string
	Content string
}

// ChatRequest is a provider-agnostic completion request.
type ChatRequest struct {
	Messages  []Message
	JSONMode  bool
	MaxTokens int
}

// ChatResponse is a provider-agnostic completion result.
type ChatResponse struct {
	Content string
}

// Provider talks to one cloud LLM.
type Provider interface {
	Name() string
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

// New builds a concrete adapter from config.
func New(cfg *config.Config) (Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("provider: nil config")
	}
	switch strings.ToLower(cfg.Provider) {
	case config.ProviderOpenAI:
		return NewOpenAI(cfg.APIKey, cfg.Model, cfg.BaseURL)
	case config.ProviderAnthropic:
		return NewAnthropic(cfg.APIKey, cfg.Model, cfg.BaseURL)
	case config.ProviderGemini:
		return NewGemini(cfg.APIKey, cfg.Model, cfg.BaseURL)
	default:
		return nil, fmt.Errorf("provider: unknown %q", cfg.Provider)
	}
}
