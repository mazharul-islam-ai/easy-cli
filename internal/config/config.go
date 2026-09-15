// Package config loads XDG and environment settings for easy-cli.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml/v2"
)

const (
	appName        = "easy-cli"
	configFileName = "config.toml"
)

// ErrNoAPIKey is returned when no provider API key is available.
var ErrNoAPIKey = errors.New("config: no API key found")

// Provider names.
const (
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"
	ProviderGemini    = "gemini"
)

// Config is the runtime configuration.
type Config struct {
	Provider     string
	Model        string
	APIKey       string
	BaseURL      string
	TimeoutSec   int
	MaxFiles     int
	MaxRank      int
	SnippetBytes int
	ConfigPath   string
	AutoConfirm  bool
}

type fileConfig struct {
	Provider struct {
		Name  string `toml:"name"`
		Model string `toml:"model"`
	} `toml:"provider"`
	Search struct {
		TimeoutSeconds int `toml:"timeout_seconds"`
		MaxFiles       int `toml:"max_files"`
		MaxRank        int `toml:"max_rank"`
		SnippetBytes   int `toml:"snippet_bytes"`
	} `toml:"search"`
	Privacy struct {
		AutoConfirm bool `toml:"auto_confirm"`
	} `toml:"privacy"`
}

// Load reads config.toml from XDG (if present) then applies environment overrides.
func Load() (*Config, error) {
	return LoadFrom(xdg.ConfigHome, os.Getenv)
}

// LoadFrom is like Load but uses an explicit config home and getenv (tests).
func LoadFrom(configHome string, getenv func(string) string) (*Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	cfg := &Config{
		TimeoutSec:   15,
		MaxFiles:     200,
		MaxRank:      20,
		SnippetBytes: 4096,
	}

	path := filepath.Join(configHome, appName, configFileName)
	cfg.ConfigPath = path
	if data, err := os.ReadFile(path); err == nil {
		var fc fileConfig
		if err := toml.Unmarshal(data, &fc); err != nil {
			return nil, fmt.Errorf("config: parse %s: %w", path, err)
		}
		if fc.Provider.Name != "" {
			cfg.Provider = fc.Provider.Name
		}
		cfg.Model = fc.Provider.Model
		if fc.Search.TimeoutSeconds > 0 {
			cfg.TimeoutSec = fc.Search.TimeoutSeconds
		}
		if fc.Search.MaxFiles > 0 {
			cfg.MaxFiles = fc.Search.MaxFiles
		}
		if fc.Search.MaxRank > 0 {
			cfg.MaxRank = fc.Search.MaxRank
		}
		if fc.Search.SnippetBytes > 0 {
			cfg.SnippetBytes = fc.Search.SnippetBytes
		}
		cfg.AutoConfirm = fc.Privacy.AutoConfirm
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	if v := strings.TrimSpace(getenv("EASY_PROVIDER")); v != "" {
		cfg.Provider = v
	}
	if v := strings.TrimSpace(getenv("EASY_MODEL")); v != "" {
		cfg.Model = v
	}
	if v := strings.TrimSpace(getenv("EASY_TIMEOUT_SECONDS")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("config: EASY_TIMEOUT_SECONDS: %w", err)
		}
		cfg.TimeoutSec = n
	}
	if truthy(getenv("EASY_YES")) || truthy(getenv("EASY_AUTO_CONFIRM")) {
		cfg.AutoConfirm = true
	}
	clampSearch(cfg)

	if strings.TrimSpace(cfg.Provider) == "" {
		cfg.Provider = firstAvailableProvider(getenv)
	}
	if strings.TrimSpace(cfg.Provider) == "" {
		return cfg, fmt.Errorf("%w (set GEMINI_API_KEY, OPENAI_API_KEY, or ANTHROPIC_API_KEY)", ErrNoAPIKey)
	}
	cfg.Provider = strings.ToLower(strings.TrimSpace(cfg.Provider))
	if err := validateProvider(cfg.Provider); err != nil {
		return nil, err
	}
	applyKeys(cfg, getenv)
	if cfg.APIKey == "" {
		return cfg, fmt.Errorf("%w for provider %s", ErrNoAPIKey, cfg.Provider)
	}
	if cfg.Model == "" {
		cfg.Model = defaultModel(cfg.Provider)
	}
	return cfg, nil
}

func applyKeys(cfg *Config, getenv func(string) string) {
	switch cfg.Provider {
	case ProviderOpenAI:
		cfg.APIKey = strings.TrimSpace(getenv("OPENAI_API_KEY"))
		cfg.BaseURL = strings.TrimSpace(getenv("OPENAI_BASE_URL"))
	case ProviderAnthropic:
		cfg.APIKey = strings.TrimSpace(getenv("ANTHROPIC_API_KEY"))
		cfg.BaseURL = strings.TrimSpace(getenv("ANTHROPIC_BASE_URL"))
	case ProviderGemini:
		cfg.APIKey = strings.TrimSpace(getenv("GEMINI_API_KEY"))
		cfg.BaseURL = strings.TrimSpace(getenv("GEMINI_BASE_URL"))
	}
}

func firstAvailableProvider(getenv func(string) string) string {
	if strings.TrimSpace(getenv("GEMINI_API_KEY")) != "" {
		return ProviderGemini
	}
	if strings.TrimSpace(getenv("OPENAI_API_KEY")) != "" {
		return ProviderOpenAI
	}
	if strings.TrimSpace(getenv("ANTHROPIC_API_KEY")) != "" {
		return ProviderAnthropic
	}
	return ""
}

func validateProvider(name string) error {
	switch name {
	case ProviderOpenAI, ProviderAnthropic, ProviderGemini:
		return nil
	default:
		return fmt.Errorf("config: unknown provider %q (use openai, anthropic, or gemini)", name)
	}
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func defaultModel(provider string) string {
	switch provider {
	case ProviderOpenAI:
		return "gpt-4.1-mini"
	case ProviderAnthropic:
		return "claude-haiku-4-5"
	case ProviderGemini:
		return "gemini-2.5-flash"
	default:
		return ""
	}
}
