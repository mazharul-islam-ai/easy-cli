package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"easy-cli/internal/config"
	"easy-cli/internal/planner"
	"easy-cli/internal/provider"
)

func TestSearchFindsInvoiceNotSecret(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".ssh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "invoice-notes.txt"), []byte("Invoice for last month. Payment terms net 30."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".ssh", "id_rsa"), []byte("fake-key"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=1"), 0o644); err != nil {
		t.Fatal(err)
	}

	invoice := filepath.Join(docs, "invoice-notes.txt")
	fake := &provider.Fake{ChatFn: func(_ context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
		joined := ""
		for _, m := range req.Messages {
			joined += m.Content
		}
		if strings.Contains(joined, "SearchPlan") {
			body, _ := json.Marshal(planner.RawPlan{
				Keywords:   []string{"invoice"},
				Extensions: []string{"txt"},
				MaxFiles:   50,
			})
			return provider.ChatResponse{Content: string(body)}, nil
		}
		body, _ := json.Marshal(map[string]any{
			"items":   []map[string]any{{"path": invoice, "score": 0.95, "reason": "invoice notes"}},
			"summary": "Found invoice notes",
		})
		return provider.ChatResponse{Content: string(body)}, nil
	}}

	a := New(&config.Config{TimeoutSec: 5, MaxFiles: 50, MaxRank: 10, SnippetBytes: 4096, AutoConfirm: true}, fake)
	var stages []Stage
	sess, err := a.SearchWithProgress(context.Background(), "invoices last month", []string{root}, func(st Stage, _ string) {
		stages = append(stages, st)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sess.Ranked.Items) != 1 || sess.Ranked.Items[0].Path != invoice {
		t.Fatalf("ranked %+v", sess.Ranked)
	}
	if len(stages) == 0 {
		t.Fatal("expected progress callbacks")
	}
	if !sess.CloudAllowed {
		t.Fatal("expected cloud ranking")
	}
	for _, h := range sess.Hits {
		if strings.Contains(h.Path, ".ssh") || strings.HasSuffix(h.Path, ".env") {
			t.Fatalf("secret hit %s", h.Path)
		}
	}
}

func TestSearchDeclineStaysLocal(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	invoice := filepath.Join(docs, "invoice-notes.txt")
	if err := os.WriteFile(invoice, []byte("Invoice for last month."), 0o644); err != nil {
		t.Fatal(err)
	}
	rankedCalls := 0
	fake := &provider.Fake{ChatFn: func(_ context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
		joined := ""
		for _, m := range req.Messages {
			joined += m.Content
		}
		if strings.Contains(joined, "PATH:") {
			rankedCalls++
		}
		if strings.Contains(joined, "SearchPlan") {
			body, _ := json.Marshal(planner.RawPlan{Keywords: []string{"invoice"}, MaxFiles: 20})
			return provider.ChatResponse{Content: string(body)}, nil
		}
		return provider.ChatResponse{Content: "{}"}, nil
	}}
	a := New(&config.Config{TimeoutSec: 5, MaxFiles: 50, MaxRank: 10, SnippetBytes: 4096}, fake)
	a.Confirm = func(context.Context, ConfirmRequest) (bool, error) { return false, nil }
	sess, err := a.Search(context.Background(), "invoices", []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if rankedCalls != 0 {
		t.Fatalf("ranker should not run, calls=%d", rankedCalls)
	}
	if sess.CloudAllowed {
		t.Fatal("cloud should be off")
	}
	if !sess.Ranked.LocalOnly {
		t.Fatal("expected local-only ranking")
	}
	if _, err := a.Ask(context.Background(), "what does it say?"); err == nil {
		t.Fatal("ask without cloud should fail")
	}
}

func TestSearchNonInteractiveNeedsYes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "invoice.txt"), []byte("invoice"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &provider.Fake{ChatFn: func(_ context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
		joined := ""
		for _, m := range req.Messages {
			joined += m.Content
		}
		if strings.Contains(joined, "SearchPlan") {
			body, _ := json.Marshal(planner.RawPlan{Keywords: []string{"invoice"}, MaxFiles: 20})
			return provider.ChatResponse{Content: string(body)}, nil
		}
		return provider.ChatResponse{Content: "{}"}, nil
	}}
	a := New(&config.Config{TimeoutSec: 5, MaxFiles: 50, MaxRank: 10, SnippetBytes: 256}, fake)
	_, err := a.Search(context.Background(), "invoices", []string{root})
	if err != ErrNeedYes {
		t.Fatalf("got %v", err)
	}
}
