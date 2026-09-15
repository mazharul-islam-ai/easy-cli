package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "easy-cli")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`
[provider]
name = "openai"
model = "file-model"

[search]
timeout_seconds = 9
max_files = 50
max_rank = 5
snippet_bytes = 1024
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	env := map[string]string{
		"EASY_PROVIDER":        "anthropic",
		"EASY_MODEL":           "claude-test",
		"ANTHROPIC_API_KEY":    "sk-ant-test",
		"OPENAI_API_KEY":       "sk-openai-ignored",
		"EASY_TIMEOUT_SECONDS": "21",
		"ANTHROPIC_BASE_URL":   "http://127.0.0.1:9",
	}
	cfg, err := LoadFrom(dir, func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != ProviderAnthropic {
		t.Fatalf("provider: got %q", cfg.Provider)
	}
	if cfg.Model != "claude-test" {
		t.Fatalf("model: got %q", cfg.Model)
	}
	if cfg.APIKey != "sk-ant-test" {
		t.Fatalf("api key mismatch")
	}
	if cfg.BaseURL != "http://127.0.0.1:9" {
		t.Fatalf("base url: got %q", cfg.BaseURL)
	}
	if cfg.TimeoutSec != 21 {
		t.Fatalf("timeout: got %d", cfg.TimeoutSec)
	}
	if cfg.MaxFiles != 50 || cfg.MaxRank != 5 || cfg.SnippetBytes != 1024 {
		t.Fatalf("search settings: %+v", cfg)
	}
}

func TestLoadFromClampsInsaneValues(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "easy-cli")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`
[search]
timeout_seconds = 99999
max_files = 999999
max_rank = 5000
snippet_bytes = 9999999
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"GEMINI_API_KEY": "g"}
	cfg, err := LoadFrom(dir, func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TimeoutSec != MaxTimeoutSec || cfg.MaxFiles != MaxFilesLimit || cfg.MaxRank != MaxRankLimit || cfg.SnippetBytes != MaxSnippetBytes {
		t.Fatalf("clamped %+v", cfg)
	}
}

func TestWriteExampleAndPrivacy(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteExample(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteExample(dir); err == nil {
		t.Fatal("expected already exists")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "API_KEY=") || strings.Contains(string(raw), "sk-") {
		t.Fatalf("example leaked a key: %s", raw)
	}
	if err := SavePrivacy(dir, Privacy{CloudSnippets: true, AcceptedAt: "now"}); err != nil {
		t.Fatal(err)
	}
	p, err := LoadPrivacy(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !p.CloudSnippets {
		t.Fatal("expected consent")
	}
}

func TestLoadFromFirstAvailableKey(t *testing.T) {
	env := map[string]string{"GEMINI_API_KEY": "g-key"}
	cfg, err := LoadFrom(t.TempDir(), func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != ProviderGemini {
		t.Fatalf("got provider %q", cfg.Provider)
	}
	if cfg.Model == "" {
		t.Fatal("expected default model")
	}
}

func TestLoadFromMissingKey(t *testing.T) {
	_, err := LoadFrom(t.TempDir(), func(string) string { return "" })
	if !errors.Is(err, ErrNoAPIKey) {
		t.Fatalf("want ErrNoAPIKey, got %v", err)
	}
}

func TestLoadFromUnknownProvider(t *testing.T) {
	env := map[string]string{
		"EASY_PROVIDER":  "not-real",
		"OPENAI_API_KEY": "x",
	}
	_, err := LoadFrom(t.TempDir(), func(k string) string { return env[k] })
	if err == nil {
		t.Fatal("expected unknown provider error")
	}
}

func TestLoadFromProviderWithoutItsKey(t *testing.T) {
	env := map[string]string{
		"EASY_PROVIDER":  "openai",
		"GEMINI_API_KEY": "only-gemini",
	}
	_, err := LoadFrom(t.TempDir(), func(k string) string { return env[k] })
	if !errors.Is(err, ErrNoAPIKey) {
		t.Fatalf("want ErrNoAPIKey, got %v", err)
	}
}
