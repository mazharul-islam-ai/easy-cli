package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// ExampleTOML is written by `easy config init`. It contains no API keys.
const ExampleTOML = `# easy-cli config — keys stay in the environment, never in this file.
# ~/.config/easy-cli/config.toml

[provider]
name = "gemini"          # openai | anthropic | gemini
model = ""               # empty = provider default

[search]
timeout_seconds = 15     # walk timeout; whole-disk walks may bump this
max_files = 200          # cap 2000
max_rank = 20            # cap 50
snippet_bytes = 4096     # cap 16384

[privacy]
# true skips the per-search “send snippets?” prompt (still refuses sudo/root).
auto_confirm = false
`

// WriteExample creates config.toml if it does not exist. Keys are never written.
func WriteExample(configHome string) (string, error) {
	dir := filepath.Join(configHome, appName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("config: mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, configFileName)
	if _, err := os.Stat(path); err == nil {
		return path, fmt.Errorf("config: %s already exists", path)
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("config: stat %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(ExampleTOML), 0o600); err != nil {
		return "", fmt.Errorf("config: write %s: %w", path, err)
	}
	return path, nil
}
