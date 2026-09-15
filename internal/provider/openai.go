package provider

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/openai/openai-go/v2/shared"
)

// OpenAI talks to the Chat Completions API.
type OpenAI struct {
	client openai.Client
	model  string
}

// NewOpenAI constructs an OpenAI adapter. baseURL may be empty.
func NewOpenAI(apiKey, model, baseURL string) (*OpenAI, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("provider: openai api key is empty")
	}
	if model == "" {
		model = "gpt-4.1-mini"
	}
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &OpenAI{client: openai.NewClient(opts...), model: model}, nil
}

// Name implements Provider.
func (o *OpenAI) Name() string { return "openai" }

// Chat implements Provider.
func (o *OpenAI) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	msgs := make([]openai.ChatCompletionMessageParamUnion, 0, len(req.Messages))
	for _, m := range req.Messages {
		switch m.Role {
		case RoleSystem:
			msgs = append(msgs, openai.SystemMessage(m.Content))
		case RoleAssistant:
			msgs = append(msgs, openai.ChatCompletionMessageParamOfAssistant(m.Content))
		default:
			msgs = append(msgs, openai.UserMessage(m.Content))
		}
	}
	params := openai.ChatCompletionNewParams{
		Messages: msgs,
		Model:    o.model,
	}
	if req.MaxTokens > 0 {
		params.MaxTokens = openai.Int(int64(req.MaxTokens))
	}
	if req.JSONMode {
		format := shared.NewResponseFormatJSONObjectParam()
		params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &format,
		}
	}
	res, err := o.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("openai: %w", err)
	}
	if len(res.Choices) == 0 {
		return ChatResponse{}, fmt.Errorf("openai: empty choices")
	}
	return ChatResponse{Content: res.Choices[0].Message.Content}, nil
}
