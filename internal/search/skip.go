package search

import (
	"path/filepath"
	"strings"
)

var skipExactDirs = []string{
	"/proc",
	"/sys",
	"/dev",
	"/run",
	"/snap",
	"/lost+found",
	"/tmp",
	"/var/tmp",
	"/var/cache",
}

var skipDirNames = map[string]struct{}{
	".ssh":            {},
	".gnupg":          {},
	".git":            {},
	"node_modules":    {},
	"lost+found":      {},
	".cache":          {},
	"__pycache__":     {},
	".aws":            {},
	".kube":           {},
	".docker":         {},
	".password-store": {},
	".azure":          {},
}

var secretPathDirs = map[string]struct{}{
	".ssh":            {},
	".gnupg":          {},
	".aws":            {},
	".kube":           {},
	".docker":         {},
	".password-store": {},
}

var secretFileNames = map[string]struct{}{
	".env":                                 {},
	"credentials.json":                     {},
	"id_rsa":                               {},
	"id_dsa":                               {},
	"id_ecdsa":                             {},
	"id_ed25519":                           {},
	"shadow":                               {},
	"gshadow":                              {},
	".netrc":                               {},
	"_netrc":                               {},
	".npmrc":                               {},
	".pypirc":                              {},
	".git-credentials":                     {},
	"application_default_credentials.json": {},
}

var secretSuffixes = []string{
	".pem", ".key", ".p12", ".pfx", ".jks", ".keystore",
}

func under(path, dir string) bool {
	path = filepath.Clean(path)
	dir = filepath.Clean(dir)
	return path == dir || strings.HasPrefix(path, dir+string(filepath.Separator))
}

// ShouldSkipDir reports whether a directory should not be descended into.
// System roots such as /tmp are skipped when walking from outside them, but
// an explicit search root inside /tmp (tests, user --scope) is still walked.
func ShouldSkipDir(path, root string) bool {
	clean := filepath.Clean(path)
	root = filepath.Clean(root)
	for _, p := range skipExactDirs {
		if under(clean, p) && !under(root, p) {
			return true
		}
	}
	base := filepath.Base(clean)
	if _, ok := skipDirNames[base]; ok {
		return true
	}
	return IsSecretPath(clean)
}

// ShouldSkipFile reports non-secret files that are never useful candidates.
func ShouldSkipFile(path string) bool {
	base := filepath.Base(path)
	return strings.HasSuffix(strings.ToLower(base), ".pyc")
}

// IsSecretPath reports paths that must never be returned or sent to an LLM.
func IsSecretPath(path string) bool {
	clean := filepath.Clean(path)
	if clean == "/etc/shadow" || clean == "/etc/gshadow" {
		return true
	}
	parts := strings.Split(clean, string(filepath.Separator))
	for i, p := range parts {
		if _, ok := secretPathDirs[p]; ok {
			return true
		}
		if p == "gcloud" && i > 0 && parts[i-1] == ".config" {
			return true
		}
	}
	base := filepath.Base(clean)
	lower := strings.ToLower(base)
	if _, ok := secretFileNames[base]; ok {
		return true
	}
	if _, ok := secretFileNames[lower]; ok {
		return true
	}
	if strings.HasPrefix(base, "id_rsa") || strings.HasPrefix(lower, "id_ed25519") ||
		strings.HasPrefix(lower, "id_dsa") || strings.HasPrefix(lower, "id_ecdsa") {
		return true
	}
	for _, suf := range secretSuffixes {
		if strings.HasSuffix(lower, suf) {
			return true
		}
	}
	if lower == ".env" || strings.HasPrefix(lower, ".env.") {
		return true
	}
	if lower == "credentials.json" || strings.HasSuffix(lower, "wallet.dat") {
		return true
	}
	if lower == "config.json" {
		for _, p := range parts {
			if p == ".docker" {
				return true
			}
		}
	}
	return false
}
