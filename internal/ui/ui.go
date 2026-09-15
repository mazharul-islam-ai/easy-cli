// Package ui is terminal chrome: colors, query box, status, result cards.
package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"

	"easy-cli/internal/app"
	"easy-cli/internal/ranker"
)

var (
	colorAccent = lipgloss.Color("51")  // cyan
	colorMuted  = lipgloss.Color("245") // gray
	colorOk     = lipgloss.Color("82")  // green
	colorWarn   = lipgloss.Color("214") // amber
	colorErr    = lipgloss.Color("203") // red
	colorPath   = lipgloss.Color("159")
	colorIndex  = lipgloss.Color("99")
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	mutedStyle  = lipgloss.NewStyle().Foreground(colorMuted)
	okStyle     = lipgloss.NewStyle().Foreground(colorOk)
	warnStyle   = lipgloss.NewStyle().Foreground(colorWarn)
	errStyle    = lipgloss.NewStyle().Foreground(colorErr).Bold(true)
	indexStyle  = lipgloss.NewStyle().Foreground(colorIndex).Bold(true)
	fileStyle   = lipgloss.NewStyle().Foreground(colorPath).Bold(true)
	dirStyle    = lipgloss.NewStyle().Foreground(colorMuted)
	reasonStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	boxStyle    = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(0, 1)
	helpStyle = lipgloss.NewStyle().Foreground(colorMuted)
)

// IsTTY reports whether w is a terminal (stdout/stderr files).
func IsTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// Header paints the session banner.
func Header(provider, scope string) string {
	if scope == "" {
		scope = strings.Join(app.DefaultRoots(), " ")
	}
	line := fmt.Sprintf(" easy  ·  %s  ·  scope %s ", provider, scope)
	return boxStyle.Render(titleStyle.Render(line) + "\n" + mutedStyle.Render("read-only search  ·  snippets need your OK before they leave this machine  ·  help  ·  ctrl+c quits"))
}

// StatusLine is a live stage message (planning / walking / LLM).
func StatusLine(spinner, detail string) string {
	if detail == "" {
		return ""
	}
	return warnStyle.Render(fmt.Sprintf("%s  %s", spinner, detail))
}

// HelpText is the command cheat-sheet.
func HelpText() string {
	return boxStyle.BorderForeground(colorMuted).Render(strings.TrimSpace(`
commands
  find / search <q>   search files
  scope <paths>       set folders (default is your home)
  scope /             whole disk as your user (not sudo)
  help                this list
  exit / ctrl+c       quit (ctrl+c during a search cancels it)

Default scope is $HOME. Whole-disk / is slower and skips other users' files.
Before ranking, easy lists the files it would send to the cloud. Type y to send,
or n for local filename matches only. Scripts: pass --yes.

Anything else is a search. After a confirmed search, questions like
"what does the first one say?" go to the model.
`))
}

// RenderConfirm lists files about to be sent to a cloud model.
func RenderConfirm(req app.ConfirmRequest) string {
	var b strings.Builder
	b.WriteString(warnStyle.Bold(true).Render("send to " + req.Provider + "?"))
	b.WriteString("\n")
	if req.FirstRun {
		b.WriteString(mutedStyle.Render("Truncated file contents leave this machine and go to the cloud provider."))
		b.WriteString("\n")
	}
	b.WriteString(mutedStyle.Render(fmt.Sprintf("%d files  ·  %d bytes of snippets", len(req.Paths), req.Bytes)))
	b.WriteString("\n")
	limit := 12
	for i, p := range req.Paths {
		if i == limit {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  … %d more", len(req.Paths)-limit)))
			b.WriteString("\n")
			break
		}
		b.WriteString("  " + fileStyle.Render(p) + "\n")
	}
	return boxStyle.BorderForeground(colorWarn).Render(strings.TrimRight(b.String(), "\n"))
}

// RenderYou is a user chat bubble.
func RenderYou(text string) string {
	st := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(0, 1)
	return st.Render(titleStyle.Render("you") + "\n" + text)
}

// RenderSession prints ranked hits as numbered path cards.
func RenderSession(sess *app.Session) string {
	if sess == nil {
		return errStyle.Render("no results")
	}
	var b strings.Builder
	summary := sess.Ranked.Summary
	if sess.Ranked.LocalOnly {
		b.WriteString(warnStyle.Render("▲  " + summary))
	} else {
		b.WriteString(okStyle.Bold(true).Render("✓  " + summary))
	}
	b.WriteString("\n")
	if sess.Stats.Denied > 0 || sess.Stats.SecretSkip > 0 || sess.Stats.SkippedDir > 0 {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("    skipped  dirs=%d  secrets=%d  denied=%d  visited=%d",
			sess.Stats.SkippedDir, sess.Stats.SecretSkip, sess.Stats.Denied, sess.Stats.Visited)))
		b.WriteString("\n")
	}
	if len(sess.Ranked.Items) == 0 {
		b.WriteString(warnStyle.Render("    No matching files."))
		b.WriteString("\n")
		return b.String()
	}
	for i, it := range sess.Ranked.Items {
		b.WriteString(renderItem(i+1, it))
	}
	return b.String()
}

func renderItem(n int, it ranker.Item) string {
	dir := filepath.Dir(it.Path)
	base := filepath.Base(it.Path)
	reason := it.Reason
	if reason == "" {
		reason = "match"
	}
	idx := indexStyle.Render(fmt.Sprintf("%2d", n))
	line := fmt.Sprintf("%s  %s  %s", idx, fileStyle.Render(base), dirStyle.Render(dir))
	return line + "\n    " + reasonStyle.Render(reason) + "\n"
}

// RenderAnswer styles a follow-up LLM answer.
func RenderAnswer(text string) string {
	return boxStyle.BorderForeground(colorOk).Render(okStyle.Render("answer") + "\n" + text)
}

// RenderError styles an error.
func RenderError(err error) string {
	if err == nil {
		return ""
	}
	return errStyle.Render("✗  " + err.Error())
}

// RenderScope confirms a scope change.
func RenderScope(paths []string) string {
	if len(paths) == 0 {
		return mutedStyle.Render("scope  " + strings.Join(app.DefaultRoots(), "  ") + "  (default home)")
	}
	return okStyle.Render("scope  " + strings.Join(paths, "  "))
}

// HintBar is the footer under the query box.
func HintBar() string {
	return helpStyle.Render("enter search   ·   ? follow-up after confirmed results   ·   help   ·   ctrl+c quit")
}

// HintBarConfirm is shown while waiting for y/N.
func HintBarConfirm() string {
	return helpStyle.Render("y send snippets to the cloud   ·   n keep local matches only   ·   ctrl+c cancel")
}
