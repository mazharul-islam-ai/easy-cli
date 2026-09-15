package config

// Hard ceilings so a bad config.toml cannot send a huge corpus to the cloud.
const (
	MaxTimeoutSec   = 300
	MaxFilesLimit   = 2000
	MaxRankLimit    = 50
	MaxSnippetBytes = 16384
	MinTimeoutSec   = 1
	MinFilesLimit   = 1
	MinRankLimit    = 1
	MinSnippetBytes = 256
)

func clampInt(v, min, max, fallback int) int {
	if v <= 0 {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func clampSearch(cfg *Config) {
	cfg.TimeoutSec = clampInt(cfg.TimeoutSec, MinTimeoutSec, MaxTimeoutSec, 15)
	cfg.MaxFiles = clampInt(cfg.MaxFiles, MinFilesLimit, MaxFilesLimit, 200)
	cfg.MaxRank = clampInt(cfg.MaxRank, MinRankLimit, MaxRankLimit, 20)
	cfg.SnippetBytes = clampInt(cfg.SnippetBytes, MinSnippetBytes, MaxSnippetBytes, 4096)
}
