package main

import (
	"os"

	"easy-cli/internal/cli"
	"easy-cli/internal/safeguard"
	"easy-cli/internal/version"
)

func main() {
	if err := safeguard.Check(os.Getenv, os.Geteuid()); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
	cmd := cli.NewRoot(cli.Options{})
	cmd.Version = version.String()
	if err := cmd.Execute(); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
