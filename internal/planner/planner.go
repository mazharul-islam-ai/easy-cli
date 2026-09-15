// Package planner turns a natural-language query into a local SearchPlan.
package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"easy-cli/internal/provider"
	"easy-cli/internal/search"
)

const systemPrompt = `You convert a user's file-search request into a single JSON object (SearchPlan).
No markdown, no commentary. Schema:
{"roots":["..."],"name_globs":["*.pdf"],"extensions":["pdf","txt"],"keywords":["invoice"],"content_regex":"","mtime_after":"","max_files":200}
Rules:
- roots: leave empty to use the caller defaults. Never invent other users' home directories.
- extensions: no leading dots. Use pdf, txt, md, docx when the user names those types.
- keywords: lowercase content words that must appear in the file (not stopwords).
- name_globs: filename globs such as "*invoice*".
- mtime_after: RFC3339 timestamp, or relative like "30d" or "24h", or empty.
- max_files: integer 1-1000, default 200.
- Do not request secret files (.env, ssh keys, pem). Leave those out.`

// Planner calls an LLM then validates JSON.
type Planner struct {
	LLM   provider.Provider
	Now   func() time.Time
	Limit int
}

// Plan returns a validated search.Plan.
func (p *Planner) Plan(ctx context.Context, query string, defaultRoots []string) (search.Plan, error) {
	if p == nil || p.LLM == nil {
		return search.Plan{}, fmt.Errorf("planner: nil provider")
	}
	now := time.Now
	if p.Now != nil {
		now = p.Now
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 200
	}
	resp, err := p.LLM.Chat(ctx, provider.ChatRequest{
		JSONMode:  true,
		MaxTokens: 800,
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: systemPrompt},
			{Role: provider.RoleUser, Content: "Default roots: " + strings.Join(defaultRoots, ", ") + "\nRequest: " + query},
		},
	})
	if err != nil {
		return search.Plan{}, err
	}
	raw, err := parseJSON(resp.Content)
	if err != nil {
		return search.Plan{}, fmt.Errorf("planner: %w", err)
	}
	plan, err := Validate(raw, defaultRoots, limit, now())
	if err != nil {
		return search.Plan{}, err
	}
	return EnrichFromQuery(plan, query), nil
}

// RawPlan is the JSON object the model should produce.
type RawPlan struct {
	Roots        []string `json:"roots"`
	NameGlobs    []string `json:"name_globs"`
	Extensions   []string `json:"extensions"`
	Keywords     []string `json:"keywords"`
	ContentRegex string   `json:"content_regex"`
	MtimeAfter   string   `json:"mtime_after"`
	MaxFiles     int      `json:"max_files"`
}

func parseJSON(s string) (RawPlan, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "{"); i >= 0 {
		if j := strings.LastIndex(s, "}"); j > i {
			s = s[i : j+1]
		}
	}
	var raw RawPlan
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return RawPlan{}, fmt.Errorf("invalid JSON: %w", err)
	}
	return raw, nil
}

// Validate applies defaults and rejects junk.
func Validate(raw RawPlan, defaultRoots []string, limit int, now time.Time) (search.Plan, error) {
	if limit <= 0 {
		limit = 200
	}
	plan := search.Plan{
		Roots:        append([]string{}, raw.Roots...),
		NameGlobs:    compact(raw.NameGlobs),
		Extensions:   compact(raw.Extensions),
		Keywords:     compact(raw.Keywords),
		ContentRegex: strings.TrimSpace(raw.ContentRegex),
		MaxFiles:     raw.MaxFiles,
	}
	if len(plan.Roots) == 0 {
		plan.Roots = append([]string{}, defaultRoots...)
	}
	if plan.MaxFiles <= 0 {
		plan.MaxFiles = limit
	}
	if plan.MaxFiles > 1000 {
		plan.MaxFiles = 1000
	}
	if raw.MtimeAfter != "" {
		t, err := parseMtime(raw.MtimeAfter, now)
		if err != nil {
			return search.Plan{}, fmt.Errorf("planner: mtime_after: %w", err)
		}
		plan.MtimeAfter = t
	}
	return plan, nil
}

func parseMtime(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return time.Time{}, err
		}
		return now.Add(-time.Duration(n) * 24 * time.Hour), nil
	}
	if strings.HasSuffix(s, "h") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "h"))
		if err != nil {
			return time.Time{}, err
		}
		return now.Add(-time.Duration(n) * time.Hour), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Time{}, err
	}
	return now.Add(-d), nil
}

func compact(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// EnrichFromQuery adds filename globs from tokens like SKILL.md so a name
// search still works even if the model over-constrained keywords.
func EnrichFromQuery(plan search.Plan, query string) search.Plan {
	for _, tok := range strings.Fields(query) {
		tok = strings.Trim(tok, `"',`)
		if tok == "" {
			continue
		}
		low := strings.ToLower(tok)
		if low == "find" || low == "search" || low == "the" || low == "from" || low == "about" {
			continue
		}
		if strings.Contains(tok, ".") && !strings.HasPrefix(tok, ".") {
			plan.NameGlobs = appendUnique(plan.NameGlobs, tok, "*"+tok+"*")
		}
	}
	return plan
}

// Relax drops content filters so a filename/glob search can succeed on a large tree.
func Relax(plan search.Plan) search.Plan {
	plan.Keywords = nil
	plan.ContentRegex = ""
	plan.MtimeAfter = time.Time{}
	return plan
}

func appendUnique(dst []string, extras ...string) []string {
	seen := map[string]struct{}{}
	for _, s := range dst {
		seen[strings.ToLower(s)] = struct{}{}
	}
	for _, s := range extras {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		k := strings.ToLower(s)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		dst = append(dst, s)
	}
	return dst
}
