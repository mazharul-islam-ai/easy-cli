// Package search walks the local filesystem read-only.
package search

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ErrTimeout is returned when the walk hits its deadline with no hits.
var ErrTimeout = errors.New("search: timed out")

// Plan is a structured local search (produced by the planner, executed here).
type Plan struct {
	Roots        []string
	NameGlobs    []string
	Extensions   []string
	Keywords     []string
	ContentRegex string
	MtimeAfter   time.Time
	MaxFiles     int
}

// Hit is a candidate file.
type Hit struct {
	Path    string
	Size    int64
	ModTime time.Time
}

// Stats records walk side effects.
type Stats struct {
	Visited    int
	SkippedDir int
	Denied     int
	SecretSkip int
}

// Run walks roots according to plan. It never follows directory symlinks.
func Run(ctx context.Context, plan Plan) ([]Hit, Stats, error) {
	if plan.MaxFiles <= 0 {
		plan.MaxFiles = 200
	}
	if plan.MaxFiles > 1000 {
		plan.MaxFiles = 1000
	}
	if len(plan.Roots) == 0 {
		plan.Roots = []string{"/"}
	}
	var rx *regexp.Regexp
	if strings.TrimSpace(plan.ContentRegex) != "" {
		compiled, err := regexp.Compile(plan.ContentRegex)
		if err != nil {
			return nil, Stats{}, err
		}
		rx = compiled
	}
	keywords := normalizeKeywords(plan.Keywords)
	exts := normalizeExts(plan.Extensions)

	var stats Stats
	hits := make([]Hit, 0, 32)
	timedOut := false

	for _, root := range plan.Roots {
		if ctx.Err() != nil {
			timedOut = true
			break
		}
		root = filepath.Clean(root)
		info, err := os.Lstat(root)
		if err != nil {
			if isDenied(err) {
				stats.Denied++
				continue
			}
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return hits, stats, err
		}
		if info.Mode()&os.ModeSymlink != 0 && info.IsDir() {
			stats.SkippedDir++
			continue
		}
		if !info.IsDir() {
			if info.Mode()&os.ModeSymlink != 0 {
				continue
			}
			h, ok, denied, secret := considerFile(root, info, plan, exts, keywords, rx)
			if denied {
				stats.Denied++
			}
			if secret {
				stats.SecretSkip++
			}
			if ok {
				hits = append(hits, h)
			}
			continue
		}

		walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				timedOut = true
				return fs.SkipAll
			}
			if err != nil {
				if isDenied(err) {
					stats.Denied++
					if d != nil && d.IsDir() {
						return fs.SkipDir
					}
					return nil
				}
				return nil
			}
			stats.Visited++
			if d.IsDir() {
				if path != root && (ShouldSkipDir(path, root) || isDirSymlink(path, d)) {
					if IsSecretPath(path) {
						stats.SecretSkip++
					} else {
						stats.SkippedDir++
					}
					return fs.SkipDir
				}
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			fi, lerr := d.Info()
			if lerr != nil {
				if isDenied(lerr) {
					stats.Denied++
				}
				return nil
			}
			h, ok, denied, secret := considerFile(path, fi, plan, exts, keywords, rx)
			if denied {
				stats.Denied++
				return nil
			}
			if secret {
				stats.SecretSkip++
				return nil
			}
			if ok {
				hits = append(hits, h)
				if len(hits) >= plan.MaxFiles {
					return fs.SkipAll
				}
			}
			return nil
		})
		if walkErr != nil && !errors.Is(walkErr, fs.SkipAll) {
			return hits, stats, walkErr
		}
		if len(hits) >= plan.MaxFiles {
			break
		}
	}

	if timedOut && len(hits) == 0 {
		return hits, stats, ErrTimeout
	}
	return hits, stats, nil
}

func considerFile(path string, info os.FileInfo, plan Plan, exts []string, keywords []string, rx *regexp.Regexp) (Hit, bool, bool, bool) {
	if IsSecretPath(path) {
		return Hit{}, false, false, true
	}
	if ShouldSkipFile(path) {
		return Hit{}, false, false, false
	}
	if !plan.MtimeAfter.IsZero() && info.ModTime().Before(plan.MtimeAfter) {
		return Hit{}, false, false, false
	}
	base := filepath.Base(path)
	if len(plan.NameGlobs) > 0 && !matchAnyGlob(base, plan.NameGlobs) {
		return Hit{}, false, false, false
	}
	if len(exts) > 0 && !matchExt(path, exts) {
		return Hit{}, false, false, false
	}
	if len(keywords) > 0 || rx != nil {
		ok, denied := contentMatches(path, keywords, rx)
		if denied {
			return Hit{}, false, true, false
		}
		if !ok {
			return Hit{}, false, false, false
		}
	}
	return Hit{Path: path, Size: info.Size(), ModTime: info.ModTime()}, true, false, false
}

func contentMatches(path string, keywords []string, rx *regexp.Regexp) (ok bool, denied bool) {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false, isDenied(err)
	}
	defer f.Close()
	buf := make([]byte, 64*1024)
	n, _ := f.Read(buf)
	if n == 0 {
		return false, false
	}
	if isLikelyBinary(buf[:n]) {
		return false, false
	}
	text := strings.ToLower(string(buf[:n]))
	for _, kw := range keywords {
		if !strings.Contains(text, kw) {
			return false, false
		}
	}
	if rx != nil && !rx.Match(buf[:n]) {
		return false, false
	}
	return true, false
}

func isLikelyBinary(b []byte) bool {
	n := len(b)
	if n > 800 {
		n = 800
	}
	for i := 0; i < n; i++ {
		if b[i] == 0 {
			return true
		}
	}
	return false
}

func matchAnyGlob(base string, globs []string) bool {
	lowerBase := strings.ToLower(base)
	for _, g := range globs {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		if ok, _ := filepath.Match(g, base); ok {
			return true
		}
		if ok, _ := filepath.Match(strings.ToLower(g), lowerBase); ok {
			return true
		}
	}
	return false
}

func matchExt(path string, exts []string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	for _, e := range exts {
		if ext == e {
			return true
		}
	}
	return false
}

func normalizeExts(exts []string) []string {
	out := make([]string, 0, len(exts))
	for _, e := range exts {
		e = strings.ToLower(strings.TrimSpace(e))
		e = strings.TrimPrefix(e, ".")
		if e != "" {
			out = append(out, e)
		}
	}
	return out
}

func normalizeKeywords(kws []string) []string {
	out := make([]string, 0, len(kws))
	for _, k := range kws {
		k = strings.ToLower(strings.TrimSpace(k))
		if k != "" {
			out = append(out, k)
		}
	}
	return out
}

func isDirSymlink(path string, d fs.DirEntry) bool {
	if d.Type()&os.ModeSymlink != 0 {
		return true
	}
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

func isDenied(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, fs.ErrPermission)
}
