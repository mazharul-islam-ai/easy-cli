package planner

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"easy-cli/internal/provider"
	"easy-cli/internal/search"
)

func TestPlanTable(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		query   string
		raw     RawPlan
		wantExt []string
		wantKW  []string
		wantAge time.Duration
	}{
		{
			name:  "invoices last month pdf",
			query: "PDFs about invoices from last month",
			raw: RawPlan{
				Extensions: []string{"pdf"},
				Keywords:   []string{"invoice"},
				MtimeAfter: "30d",
				MaxFiles:   200,
			},
			wantExt: []string{"pdf"},
			wantKW:  []string{"invoice"},
			wantAge: 30 * 24 * time.Hour,
		},
		{
			name:  "markdown notes",
			query: "find markdown notes named readme",
			raw: RawPlan{
				NameGlobs:  []string{"README*"},
				Extensions: []string{"md"},
			},
			wantExt: []string{"md"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.raw)
			fake := &provider.Fake{ChatFn: func(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
				return provider.ChatResponse{Content: string(body)}, nil
			}}
			p := &Planner{LLM: fake, Now: func() time.Time { return now }, Limit: 200}
			plan, err := p.Plan(context.Background(), tt.query, []string{"/home/u"})
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Roots) != 1 || plan.Roots[0] != "/home/u" {
				t.Fatalf("roots %+v", plan.Roots)
			}
			if !equal(plan.Extensions, tt.wantExt) {
				t.Fatalf("ext %+v", plan.Extensions)
			}
			if tt.wantKW != nil && !equal(plan.Keywords, tt.wantKW) {
				t.Fatalf("kw %+v", plan.Keywords)
			}
			if tt.wantAge != 0 {
				got := now.Sub(plan.MtimeAfter)
				if got < tt.wantAge-time.Second || got > tt.wantAge+time.Second {
					t.Fatalf("mtime delta %v want %v", got, tt.wantAge)
				}
			}
		})
	}
}

func TestInvalidJSON(t *testing.T) {
	fake := &provider.Fake{ChatFn: func(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
		return provider.ChatResponse{Content: "not json"}, nil
	}}
	p := &Planner{LLM: fake}
	_, err := p.Plan(context.Background(), "x", []string{"/"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFencedJSON(t *testing.T) {
	fake := &provider.Fake{ChatFn: func(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
		return provider.ChatResponse{Content: "```json\n{\"keywords\":[\"invoice\"],\"max_files\":10}\n```"}, nil
	}}
	p := &Planner{LLM: fake}
	plan, err := p.Plan(context.Background(), "invoice", []string{"/tmp"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.MaxFiles != 10 || plan.Keywords[0] != "invoice" {
		t.Fatalf("%+v", plan)
	}
}

func TestValidateCapsMaxFiles(t *testing.T) {
	plan, err := Validate(RawPlan{MaxFiles: 5000}, []string{"/"}, 200, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if plan.MaxFiles != 1000 {
		t.Fatalf("got %d", plan.MaxFiles)
	}
}

func TestEnrichFromQueryAddsFilenameGlob(t *testing.T) {
	plan := EnrichFromQuery(search.Plan{Keywords: []string{"skill"}}, "find SKILL.md")
	found := false
	for _, g := range plan.NameGlobs {
		if g == "SKILL.md" || g == "*SKILL.md*" {
			found = true
		}
	}
	if !found {
		t.Fatalf("globs=%v", plan.NameGlobs)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
