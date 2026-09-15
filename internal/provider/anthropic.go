package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Anthropic talks to the Messages API.
type Anthropic struct {
	client anthropic.Client
	model  anthropic.Model
}

// NewAnthropic constructs a Claude adapter. baseURL may be empty.
func NewAnthropic(apiKey, model, baseURL string) (*Anthropic, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("provider: anthropic api key is empty")
	}
	if model == "" {
		model = "claude-haiku-4-5"
	}
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &Anthropic{client: anthropic.NewClient(opts...), model: anthropic.Model(model)}, nil
}

// Name implements Provider.
func (a *Anthropic) Name() string { return "anthropic" }

// Chat implements Provider.
func (a *Anthropic) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	var system []anthropic.TextBlockParam
	var msgs []anthropic.MessageParam
	for _, m := range req.Messages {
		switch m.Role {
		case RoleSystem:
			system = append(system, anthropic.TextBlockParam{Text: m.Content})
		case RoleAssistant:
			msgs = append(msgs, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		default:
			msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		}
	}
	if len(msgs) == 0 {
		return ChatResponse{}, fmt.Errorf("anthropic: no user messages")
	}
	maxTok := int64(req.MaxTokens)
	if maxTok <= 0 {
		maxTok = 1024
	}
	params := anthropic.MessageNewParams{
		Model:     a.model,
		MaxTokens: maxTok,
		Messages:  msgs,
		System:    system,
	}
	if req.JSONMode {
		params.System = append(params.System, anthropic.TextBlockParam{
			Text: "Reply with a single JSON object and no markdown.",
		})
	}
	res, err := a.client.Messages.New(ctx, params)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("anthropic: %w", err)
	}
	var b strings.Builder
	for _, c := range res.Content {
		if c.Text != "" {
			b.WriteString(c.Text)
		}
	}
	return ChatResponse{Content: b.String()}, nil
}
