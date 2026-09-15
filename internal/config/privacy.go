package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

const (
	privacyFileName = "privacy.toml"
)

// Privacy is first-run consent stored next to config.toml (mode 0600).
type Privacy struct {
	CloudSnippets bool   `toml:"cloud_snippets"`
	AcceptedAt    string `toml:"accepted_at"`
}

// PrivacyPath is ~/.config/easy-cli/privacy.toml under configHome.
func PrivacyPath(configHome string) string {
	return filepath.Join(configHome, appName, privacyFileName)
}

// LoadPrivacy reads consent. Missing file is empty Privacy, not an error.
func LoadPrivacy(configHome string) (Privacy, error) {
	path := PrivacyPath(configHome)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Privacy{}, nil
		}
		return Privacy{}, fmt.Errorf("config: read %s: %w", path, err)
	}
	var p Privacy
	if err := toml.Unmarshal(data, &p); err != nil {
		return Privacy{}, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return p, nil
}

// SavePrivacy writes consent with directory 0700 and file 0600. It never stores API keys.
func SavePrivacy(configHome string, p Privacy) error {
	dir := filepath.Join(configHome, appName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("config: mkdir %s: %w", dir, err)
	}
	data, err := toml.Marshal(p)
	if err != nil {
		return fmt.Errorf("config: marshal privacy: %w", err)
	}
	path := PrivacyPath(configHome)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	return nil
}
