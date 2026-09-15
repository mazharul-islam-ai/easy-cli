package ui

import (
	"strings"
	"testing"

	"easy-cli/internal/app"
	"easy-cli/internal/ranker"
	"easy-cli/internal/search"
)

func TestRenderSessionIncludesPath(t *testing.T) {
	s := RenderSession(&app.Session{
		Ranked: ranker.Result{Summary: "Found invoice", Items: []ranker.Item{
			{Path: "/docs/invoice-notes.txt", Reason: "matches query"},
		}},
		Stats: search.Stats{Visited: 3},
	})
	if !strings.Contains(s, "invoice-notes.txt") {
		t.Fatalf("%s", s)
	}
	if !strings.Contains(s, "Found invoice") {
		t.Fatalf("%s", s)
	}
}

func TestRenderConfirm(t *testing.T) {
	s := RenderConfirm(app.ConfirmRequest{
		Provider: "gemini",
		Paths:    []string{"/docs/a.txt"},
		Bytes:    12,
		FirstRun: true,
	})
	if !strings.Contains(s, "gemini") || !strings.Contains(s, "/docs/a.txt") {
		t.Fatalf("%s", s)
	}
}

func TestRenderError(t *testing.T) {
	if !strings.Contains(RenderError(errSample), "boom") {
		t.Fatal(RenderError(errSample))
	}
}

func TestRenderYou(t *testing.T) {
	s := RenderYou("find SKILL.md")
	if !strings.Contains(s, "find SKILL.md") || !strings.Contains(s, "you") {
		t.Fatalf("%s", s)
	}
}

type sampleErr struct{}

func (sampleErr) Error() string { return "boom" }

var errSample error = sampleErr{}
