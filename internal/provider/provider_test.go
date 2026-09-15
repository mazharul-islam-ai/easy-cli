package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "hello") {
			t.Errorf("body=%s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-1",
			"object":  "chat.completion",
			"created": 1,
			"model":   "gpt-4.1-mini",
			"choices": []map[string]any{{
				"index":         0,
				"finish_reason": "stop",
				"message": map[string]any{
					"role":    "assistant",
					"content": `{"ok":true}`,
				},
			}},
		})
	}))
	t.Cleanup(srv.Close)

	p, err := NewOpenAI("sk-test", "gpt-4.1-mini", srv.URL+"/v1")
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
		JSONMode: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Content != `{"ok":true}` {
		t.Fatalf("got %q", res.Content)
	}
}

func TestAnthropicChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":            "msg_1",
			"type":          "message",
			"role":          "assistant",
			"model":         "claude-haiku-4-5",
			"content":       []map[string]any{{"type": "text", "text": "pong"}},
			"stop_reason":   "end_turn",
			"stop_sequence": nil,
			"usage":         map[string]any{"input_tokens": 1, "output_tokens": 1},
		})
	}))
	t.Cleanup(srv.Close)

	p, err := NewAnthropic("sk-ant-test", "claude-haiku-4-5", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Chat(context.Background(), ChatRequest{
		Messages: []Message{
			{Role: RoleSystem, Content: "be brief"},
			{Role: RoleUser, Content: "ping"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Content != "pong" {
		t.Fatalf("got %q", res.Content)
	}
}

func TestGeminiChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{{
				"content": map[string]any{
					"role":  "model",
					"parts": []map[string]any{{"text": "gemini-ok"}},
				},
				"finishReason": "STOP",
			}},
		})
	}))
	t.Cleanup(srv.Close)

	p, err := NewGemini("g-key", "gemini-2.5-flash", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Content != "gemini-ok" {
		t.Fatalf("got %q", res.Content)
	}
}

func TestNewUnknown(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
