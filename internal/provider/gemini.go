package provider

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

// Gemini talks to the Gemini API.
type Gemini struct {
	client *genai.Client
	model  string
}

// NewGemini constructs a Gemini adapter. baseURL may be empty.
func NewGemini(apiKey, model, baseURL string) (*Gemini, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("provider: gemini api key is empty")
	}
	if model == "" {
		model = "gemini-2.5-flash"
	}
	cfg := &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	}
	if baseURL != "" {
		cfg.HTTPOptions.BaseURL = baseURL
	}
	client, err := genai.NewClient(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("gemini: client: %w", err)
	}
	return &Gemini{client: client, model: model}, nil
}

// Name implements Provider.
func (g *Gemini) Name() string { return "gemini" }

// Chat implements Provider.
func (g *Gemini) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	var sysParts []string
	var contents []*genai.Content
	for _, m := range req.Messages {
		switch m.Role {
		case RoleSystem:
			sysParts = append(sysParts, m.Content)
		case RoleAssistant:
			contents = append(contents, genai.NewContentFromText(m.Content, genai.RoleModel))
		default:
			contents = append(contents, genai.NewContentFromText(m.Content, genai.RoleUser))
		}
	}
	if len(contents) == 0 {
		return ChatResponse{}, fmt.Errorf("gemini: no user messages")
	}
	cfg := &genai.GenerateContentConfig{}
	if len(sysParts) > 0 {
		cfg.SystemInstruction = genai.NewContentFromText(strings.Join(sysParts, "\n"), genai.RoleUser)
	}
	if req.JSONMode {
		cfg.ResponseMIMEType = "application/json"
	}
	if req.MaxTokens > 0 {
		cfg.MaxOutputTokens = int32(req.MaxTokens)
	}
	res, err := g.client.Models.GenerateContent(ctx, g.model, contents, cfg)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("gemini: %w", err)
	}
	return ChatResponse{Content: res.Text()}, nil
}
