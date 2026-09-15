package app

import (
	"os"
	"strings"
)

// DefaultRoots is the production default search scope: the current user's home.
func DefaultRoots() []string {
	h, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(h) == "" {
		return []string{"."}
	}
	return []string{h}
}

func resolveRoots(roots []string) []string {
	if len(roots) == 0 {
		return DefaultRoots()
	}
	out := make([]string, 0, len(roots))
	for _, r := range roots {
		r = strings.TrimSpace(r)
		if r != "" {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return DefaultRoots()
	}
	return out
}
