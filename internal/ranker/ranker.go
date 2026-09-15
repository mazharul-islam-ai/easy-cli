// Package ranker asks an LLM to order and explain local search hits.
package ranker

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"easy-cli/internal/extract"
	"easy-cli/internal/provider"
	"easy-cli/internal/search"
)

const rankSystem = `You rank local files for a user query. Reply with one JSON object, no markdown:
{"items":[{"path":"/abs/path","score":0.9,"reason":"why"}],"summary":"one sentence"}
Rules:
- Only use paths from the provided list. Do not invent paths.
- Prefer documents that actually match the query.
- Never mention or include secret files.`

const askSystem = `You answer questions using only the provided file snippets.
Cite file paths. If the snippets are insufficient, say so. Do not invent contents.`

// Item is one ranked file.
type Item struct {
	Path   string  `json:"path"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

// Result is the ranked list plus a short summary.
type Result struct {
	Items     []Item `json:"items"`
	Summary   string `json:"summary"`
	LocalOnly bool   `json:"-"`
}

// Ranker uses an LLM to order snippets.
type Ranker struct {
	LLM provider.Provider
}

// Rank orders hits using snippets. allowed is the set of real hit paths.
func (r *Ranker) Rank(ctx context.Context, query string, snippets []extract.Snippet, allowed []search.Hit) (Result, error) {
	if r == nil || r.LLM == nil {
		return fallback(snippets, "no provider"), nil
	}
	allow := map[string]struct{}{}
	for _, h := range allowed {
		allow[h.Path] = struct{}{}
	}
	var b strings.Builder
	b.WriteString("Query: ")
	b.WriteString(query)
	b.WriteString("\nFiles:\n")
	for _, s := range snippets {
		if s.Skipped != "" {
			continue
		}
		fmt.Fprintf(&b, "PATH: %s\n%s\n---\n", s.Path, s.Text)
	}
	resp, err := r.LLM.Chat(ctx, provider.ChatRequest{
		JSONMode:  true,
		MaxTokens: 1200,
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: rankSystem},
			{Role: provider.RoleUser, Content: b.String()},
		},
	})
	if err != nil {
		return fallback(snippets, "ranker fallback: "+err.Error()), nil
	}
	raw := stripJSON(resp.Content)
	var out Result
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return fallback(snippets, fmt.Sprintf("%d local matches (model ranking unavailable)", countUsable(snippets))), nil
	}
	filtered := out.Items[:0]
	byBase := map[string][]string{}
	for p := range allow {
		byBase[filepath.Base(p)] = append(byBase[filepath.Base(p)], p)
	}
	seen := map[string]struct{}{}
	for _, it := range out.Items {
		path := it.Path
		if _, ok := allow[path]; !ok {
			cands := byBase[filepath.Base(path)]
			if len(cands) == 1 {
				path = cands[0]
				it.Path = path
			} else {
				continue
			}
		}
		if search.IsSecretPath(path) {
			continue
		}
		if _, dup := seen[path]; dup {
			continue
		}
		seen[path] = struct{}{}
		filtered = append(filtered, it)
	}
	out.Items = filtered
	if len(out.Items) == 0 {
		fb := fallback(snippets, out.Summary)
		if strings.TrimSpace(out.Summary) == "" {
			fb.Summary = fmt.Sprintf("%d local matches", len(fb.Items))
		}
		return fb, nil
	}
	return out, nil
}

// Ask answers a follow-up using the last snippet set.
func (r *Ranker) Ask(ctx context.Context, question string, snippets []extract.Snippet) (string, error) {
	if r == nil || r.LLM == nil {
		return "", fmt.Errorf("ranker: nil provider")
	}
	var b strings.Builder
	for _, s := range snippets {
		if s.Skipped != "" || search.IsSecretPath(s.Path) {
			continue
		}
		fmt.Fprintf(&b, "PATH: %s\n%s\n---\n", s.Path, s.Text)
	}
	resp, err := r.LLM.Chat(ctx, provider.ChatRequest{
		MaxTokens: 800,
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: askSystem},
			{Role: provider.RoleUser, Content: b.String() + "\nQuestion: " + question},
		},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

func fallback(snippets []extract.Snippet, summary string) Result {
	items := make([]Item, 0, len(snippets))
	for _, s := range snippets {
		if s.Skipped != "" || search.IsSecretPath(s.Path) {
			continue
		}
		items = append(items, Item{Path: s.Path, Score: 0, Reason: "local filename/content match"})
	}
	if summary == "" {
		summary = fmt.Sprintf("%d local matches", len(items))
	}
	return Result{Items: items, Summary: summary, LocalOnly: true}
}

// Fallback is local ranking without calling a model.
func Fallback(snippets []extract.Snippet, summary string) Result {
	return fallback(snippets, summary)
}

func countUsable(snippets []extract.Snippet) int {
	n := 0
	for _, s := range snippets {
		if s.Skipped == "" && !search.IsSecretPath(s.Path) {
			n++
		}
	}
	return n
}

func stripJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "{"); i >= 0 {
		if j := strings.LastIndex(s, "}"); j > i {
			return s[i : j+1]
		}
	}
	return s
}
