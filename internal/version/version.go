// Package version is filled at build time via ldflags.
package version

import "fmt"

// These are overridden by GoReleaser / make build.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String is the human-readable version line.
func String() string {
	return fmt.Sprintf("%s (%s %s)", Version, Commit, Date)
}
