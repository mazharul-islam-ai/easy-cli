package ranker

import (
	"context"
	"testing"

	"easy-cli/internal/extract"
	"easy-cli/internal/provider"
	"easy-cli/internal/search"
)

func TestRankKeepsOnlyAllowedPaths(t *testing.T) {
	keep := "/tmp/docs/invoice.txt"
	drop := "/tmp/docs/unrelated.txt"
	secret := "/tmp/.env"
	fake := &provider.Fake{ChatFn: func(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
		return provider.ChatResponse{Content: `{"items":[{"path":"` + keep + `","score":0.9,"reason":"invoice"},{"path":"` + drop + `","score":0.1,"reason":"no"},{"path":"` + secret + `","score":1,"reason":"secret"}],"summary":"found invoice"}`}, nil
	}}
	r := &Ranker{LLM: fake}
	out, err := r.Rank(context.Background(), "invoice", []extract.Snippet{
		{Path: keep, Text: "Invoice net 30"},
	}, []search.Hit{{Path: keep}})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 1 || out.Items[0].Path != keep {
		t.Fatalf("%+v", out.Items)
	}
	if out.Summary != "found invoice" {
		t.Fatalf("summary %q", out.Summary)
	}
}

func TestRankFallbackOnBadJSON(t *testing.T) {
	fake := &provider.Fake{ChatFn: func(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
		return provider.ChatResponse{Content: "oops"}, nil
	}}
	r := &Ranker{LLM: fake}
	p := "/tmp/a.txt"
	out, err := r.Rank(context.Background(), "q", []extract.Snippet{{Path: p, Text: "hi"}}, []search.Hit{{Path: p}})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 1 || out.Items[0].Path != p {
		t.Fatalf("%+v", out)
	}
}

func TestAsk(t *testing.T) {
	fake := &provider.Fake{ChatFn: func(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
		return provider.ChatResponse{Content: "Payment terms are net 30."}, nil
	}}
	r := &Ranker{LLM: fake}
	ans, err := r.Ask(context.Background(), "payment terms?", []extract.Snippet{
		{Path: "/tmp/invoice.txt", Text: "net 30"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ans != "Payment terms are net 30." {
		t.Fatalf("%q", ans)
	}
}
