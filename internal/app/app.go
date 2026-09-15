package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"easy-cli/internal/config"
	"easy-cli/internal/extract"
	"easy-cli/internal/planner"
	"easy-cli/internal/provider"
	"easy-cli/internal/ranker"
	"easy-cli/internal/search"
)

// ErrNeedYes is returned when snippets would be sent from a non-interactive session.
var ErrNeedYes = errors.New("refusing to send file snippets to the cloud from a non-interactive session; re-run with --yes (or EASY_YES=1) if you intend that")

// ConfirmRequest is shown before snippets are sent to a cloud model.
type ConfirmRequest struct {
	Provider string
	Paths    []string
	Bytes    int
	FirstRun bool
}

// Confirmer decides whether ranking/Q&A may send snippets to the cloud.
// Returning false is not an error: the app falls back to local ranking.
type Confirmer func(ctx context.Context, req ConfirmRequest) (bool, error)

// Stage is a pipeline step shown in the UI.
type Stage string

const (
	StagePlanning   Stage = "planning"
	StageWalking    Stage = "walking"
	StageExtracting Stage = "extracting"
	StageRanking    Stage = "ranking"
	StageAsking     Stage = "asking"
	StageDone       Stage = "done"
)

// Progress reports user-visible status. It must be safe to call with a nil Progress.
type Progress func(stage Stage, detail string)

func (p Progress) report(stage Stage, detail string) {
	if p != nil {
		p(stage, detail)
	}
}

// App is the composition root used by CLI and REPL.
type App struct {
	Cfg         *config.Config
	LLM         provider.Provider
	Planner     *planner.Planner
	Ranker      *ranker.Ranker
	Last        *Session
	AutoConfirm bool
	Confirm     Confirmer
	PrivacyOK   bool
}

// Session is the last search, for follow-up questions.
type Session struct {
	Query        string
	Plan         search.Plan
	Hits         []search.Hit
	Snippets     []extract.Snippet
	Ranked       ranker.Result
	Stats        search.Stats
	CloudAllowed bool
}

// New wires planner and ranker around a provider.
func New(cfg *config.Config, llm provider.Provider) *App {
	a := &App{
		Cfg:     cfg,
		LLM:     llm,
		Planner: &planner.Planner{LLM: llm, Limit: cfg.MaxFiles},
		Ranker:  &ranker.Ranker{LLM: llm},
	}
	if cfg != nil {
		a.AutoConfirm = cfg.AutoConfirm
	}
	return a
}

func (a *App) providerName() string {
	if a != nil && a.LLM != nil {
		return a.LLM.Name()
	}
	return "llm"
}

// Search runs the full pipeline. Empty roots use DefaultRoots ($HOME).
func (a *App) Search(ctx context.Context, query string, roots []string) (*Session, error) {
	return a.SearchWithProgress(ctx, query, roots, nil)
}

// SearchWithProgress is Search plus UI status callbacks.
func (a *App) SearchWithProgress(ctx context.Context, query string, roots []string, progress Progress) (*Session, error) {
	if a.Cfg == nil {
		return nil, fmt.Errorf("app: nil config")
	}
	roots = resolveRoots(roots)

	progress.report(StagePlanning, "Asking "+a.providerName()+" to plan the search…")
	plan, err := a.Planner.Plan(ctx, query, roots)
	if err != nil {
		return nil, err
	}
	if len(roots) > 0 {
		plan.Roots = roots
	}
	if plan.MaxFiles <= 0 || plan.MaxFiles > a.Cfg.MaxFiles {
		plan.MaxFiles = a.Cfg.MaxFiles
	}

	walkTimeout := time.Duration(a.Cfg.TimeoutSec) * time.Second
	if walkTimeout <= 0 {
		walkTimeout = 15 * time.Second
	}
	if isWholeDisk(plan.Roots) && walkTimeout < 45*time.Second {
		walkTimeout = 45 * time.Second
	}
	progress.report(StageWalking, fmt.Sprintf("Scanning %s (read-only, up to %s)…", strings.Join(plan.Roots, " "), walkTimeout))
	walkCtx, cancel := context.WithTimeout(ctx, walkTimeout)
	hits, stats, err := search.Run(walkCtx, plan)
	cancel()
	if len(hits) == 0 {
		loose := planner.Relax(plan)
		progress.report(StageWalking, "No hits yet — retrying with filename match only…")
		walkCtx, cancel = context.WithTimeout(ctx, walkTimeout)
		var stats2 search.Stats
		var err2 error
		hits, stats2, err2 = search.Run(walkCtx, loose)
		cancel()
		stats.Visited += stats2.Visited
		stats.SkippedDir += stats2.SkippedDir
		stats.Denied += stats2.Denied
		stats.SecretSkip += stats2.SecretSkip
		if err2 != nil && len(hits) == 0 {
			if err != nil {
				return nil, err
			}
			return nil, err2
		}
		plan = loose
		err = nil
	}
	if err != nil && len(hits) == 0 {
		return nil, err
	}

	limit := a.Cfg.MaxRank
	if limit <= 0 {
		limit = 20
	}
	snipCap := limit
	if snipCap < 20 {
		snipCap = 20
	}
	if snipCap > len(hits) {
		snipCap = len(hits)
	}
	progress.report(StageExtracting, fmt.Sprintf("Reading %d file snippets…", snipCap))
	snippets := make([]extract.Snippet, 0, snipCap)
	for i := 0; i < snipCap; i++ {
		if search.IsSecretPath(hits[i].Path) {
			continue
		}
		snippets = append(snippets, extract.File(hits[i].Path, a.Cfg.SnippetBytes))
	}

	var ranked ranker.Result
	cloudOK := false
	if len(snippets) == 0 {
		ranked = ranker.Result{
			Summary:   "No matching files in this scope.",
			LocalOnly: true,
		}
	} else {
		ok, err := a.allowCloud(ctx, snippets)
		if err != nil {
			return nil, err
		}
		if !ok {
			progress.report(StageRanking, "Skipping cloud ranking (local matches only)…")
			ranked = ranker.Fallback(snippets, "You declined sending snippets. Showing local filename matches.")
		} else {
			progress.report(StageRanking, "Ranking matches with "+a.providerName()+"…")
			ranked, err = a.Ranker.Rank(ctx, query, snippets, hits)
			if err != nil {
				return nil, err
			}
			cloudOK = true
		}
	}
	if len(ranked.Items) > limit {
		ranked.Items = ranked.Items[:limit]
	}

	sess := &Session{
		Query:        query,
		Plan:         plan,
		Hits:         hits,
		Snippets:     snippets,
		Ranked:       ranked,
		Stats:        stats,
		CloudAllowed: cloudOK,
	}
	a.Last = sess
	progress.report(StageDone, "")
	return sess, nil
}

func isWholeDisk(roots []string) bool {
	for _, r := range roots {
		if filepath.Clean(r) == "/" {
			return true
		}
	}
	return false
}

func (a *App) allowCloud(ctx context.Context, snippets []extract.Snippet) (bool, error) {
	if a.AutoConfirm {
		return true, nil
	}
	if a.Confirm == nil {
		return false, ErrNeedYes
	}
	req := ConfirmRequest{
		Provider: a.providerName(),
		FirstRun: !a.PrivacyOK,
	}
	for _, s := range snippets {
		if s.Skipped != "" || search.IsSecretPath(s.Path) {
			continue
		}
		req.Paths = append(req.Paths, s.Path)
		req.Bytes += len(s.Text)
	}
	if len(req.Paths) == 0 {
		return false, nil
	}
	ok, err := a.Confirm(ctx, req)
	if err != nil {
		return false, err
	}
	if ok {
		a.PrivacyOK = true
	}
	return ok, nil
}

// Ask answers a question about the last session.
func (a *App) Ask(ctx context.Context, question string) (string, error) {
	return a.AskWithProgress(ctx, question, nil)
}

// AskWithProgress is Ask plus UI status callbacks.
func (a *App) AskWithProgress(ctx context.Context, question string, progress Progress) (string, error) {
	if a.Last == nil {
		return "", fmt.Errorf("app: no previous search; run a search first")
	}
	if !a.Last.CloudAllowed {
		return "", fmt.Errorf("app: last search did not send snippets to the cloud; confirm a search first")
	}
	progress.report(StageAsking, "Asking "+a.providerName()+" about the last results…")
	ans, err := a.Ranker.Ask(ctx, question, a.Last.Snippets)
	if err != nil {
		return "", err
	}
	progress.report(StageDone, "")
	return ans, nil
}
