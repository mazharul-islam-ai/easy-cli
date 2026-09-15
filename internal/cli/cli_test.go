package cli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"easy-cli/internal/cli"
	"easy-cli/internal/config"
)

func TestSearchHelp(t *testing.T) {
	var out bytes.Buffer
	cmd := cli.NewRoot(cli.Options{Stdout: &out, Stderr: &out, ConfigHome: t.TempDir()})
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"search", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "One-shot") {
		t.Fatalf("help=%s", out.String())
	}
}

func TestMissingKey(t *testing.T) {
	var errBuf bytes.Buffer
	cmd := cli.NewRoot(cli.Options{
		Stdout:     &errBuf,
		Stderr:     &errBuf,
		ConfigHome: t.TempDir(),
		Getenv:     func(string) string { return "" },
	})
	cmd.SetArgs([]string{"search", "invoices"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing key")
	}
	if !errors.Is(err, config.ErrNoAPIKey) {
		t.Fatalf("got %v", err)
	}
}

func TestVersion(t *testing.T) {
	var out bytes.Buffer
	cmd := cli.NewRoot(cli.Options{Stdout: &out, Stderr: &out, ConfigHome: t.TempDir()})
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "dev") {
		t.Fatalf("version=%s", out.String())
	}
}

func TestConfigInit(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	cmd := cli.NewRoot(cli.Options{Stdout: &out, Stderr: &out, ConfigHome: dir})
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"config", "init"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "wrote") {
		t.Fatalf("%s", out.String())
	}
}
