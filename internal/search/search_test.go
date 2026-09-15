package search

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestIsSecretPath(t *testing.T) {
	tests := []struct {
		path   string
		secret bool
	}{
		{"/home/u/.ssh/id_rsa", true},
		{"/home/u/.gnupg/trustdb.gpg", true},
		{"/etc/shadow", true},
		{"/etc/gshadow", true},
		{"/tmp/foo.env", false},
		{"/proj/.env", true},
		{"/proj/.env.local", true},
		{"/proj/credentials.json", true},
		{"/proj/cert.pem", true},
		{"/proj/id_rsa.pub", true},
		{"/docs/invoice.txt", false},
		{"/var/log/syslog", false},
		{"/home/u/.aws/credentials", true},
		{"/home/u/.kube/config", true},
		{"/home/u/.docker/config.json", true},
		{"/home/u/.netrc", true},
		{"/home/u/.npmrc", true},
		{"/home/u/.pypirc", true},
		{"/home/u/.git-credentials", true},
		{"/home/u/.password-store/foo.gpg", true},
		{"/home/u/.config/gcloud/application_default_credentials.json", true},
		{"/proj/cert.p12", true},
		{"/proj/store.pfx", true},
		{"/home/u/Documents/notes.txt", false},
	}
	for _, tt := range tests {
		if got := IsSecretPath(tt.path); got != tt.secret {
			t.Errorf("IsSecretPath(%q)=%v want %v", tt.path, got, tt.secret)
		}
	}
}

func TestShouldSkipDir(t *testing.T) {
	if !ShouldSkipDir("/proc", "/") || !ShouldSkipDir("/proc/1", "/") {
		t.Fatal("expected /proc skipped")
	}
	if !ShouldSkipDir("/sys", "/") || !ShouldSkipDir("/dev", "/") || !ShouldSkipDir("/run", "/") {
		t.Fatal("expected virtual dirs skipped")
	}
	if !ShouldSkipDir("/tmp", "/") || !ShouldSkipDir("/tmp/x", "/") {
		t.Fatal("expected /tmp skipped from /")
	}
	if ShouldSkipDir("/tmp/gotest/docs", "/tmp/gotest") {
		t.Fatal("explicit root inside /tmp should be walked")
	}
	if !ShouldSkipDir("/home/u/.ssh", "/") {
		t.Fatal("expected .ssh skipped")
	}
	if !ShouldSkipDir("/home/u/.aws", "/") || !ShouldSkipDir("/home/u/.kube", "/") {
		t.Fatal("expected cloud credential dirs skipped")
	}
	if ShouldSkipDir("/home/u/Documents", "/") {
		t.Fatal("Documents should not be skipped")
	}
}

func TestRunFindsInvoiceSkipsSecrets(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	secrets := filepath.Join(root, "secrets")
	ssh := filepath.Join(root, ".ssh")
	hidden := filepath.Join(root, ".cache")
	mustMkdir(t, docs, secrets, ssh, hidden)
	mustWrite(t, filepath.Join(docs, "invoice-notes.txt"), "Invoice for last month. Payment terms net 30.")
	mustWrite(t, filepath.Join(docs, "readme.md"), "project notes only")
	mustWrite(t, filepath.Join(secrets, ".env"), "SECRET=1")
	mustWrite(t, filepath.Join(ssh, "id_rsa"), "-----BEGIN FAKE-----")
	mustWrite(t, filepath.Join(hidden, "junk.txt"), "invoice word inside cache dir")
	mustWrite(t, filepath.Join(docs, "old.log"), "invoice")
	old := filepath.Join(docs, "old.log")
	past := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}

	hits, stats, err := Run(context.Background(), Plan{
		Roots:      []string{root},
		Keywords:   []string{"invoice"},
		MaxFiles:   50,
		MtimeAfter: time.Now().Add(-24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("hits=%d stats=%+v paths=%v", len(hits), stats, paths(hits))
	}
	if filepath.Base(hits[0].Path) != "invoice-notes.txt" {
		t.Fatalf("got %s", hits[0].Path)
	}
	if stats.SecretSkip == 0 {
		t.Fatalf("expected secret skips, stats=%+v", stats)
	}
	for _, h := range hits {
		if IsSecretPath(h.Path) {
			t.Fatalf("secret leaked: %s", h.Path)
		}
	}
}

func TestRunGlobAndExt(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "invoice.pdf"), "pdf")
	mustWrite(t, filepath.Join(root, "invoice.txt"), "txt")
	mustWrite(t, filepath.Join(root, "other.pdf"), "pdf")
	hits, _, err := Run(context.Background(), Plan{
		Roots:      []string{root},
		NameGlobs:  []string{"invoice.*"},
		Extensions: []string{"pdf"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || filepath.Base(hits[0].Path) != "invoice.pdf" {
		t.Fatalf("got %v", paths(hits))
	}
}

func TestRunSkipsDirSymlink(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	mustMkdir(t, realDir)
	mustWrite(t, filepath.Join(realDir, "secret-invoice.txt"), "invoice")
	if err := os.Symlink(realDir, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "visible.txt"), "invoice")
	hits, _, err := Run(context.Background(), Plan{
		Roots:    []string{root},
		Keywords: []string{"invoice"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hits {
		if strings.Contains(h.Path, string(filepath.Separator)+"link"+string(filepath.Separator)) {
			t.Fatalf("followed symlink dir: %s", h.Path)
		}
	}
}

func TestRunTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := Run(ctx, Plan{Roots: []string{t.TempDir()}})
	if err != ErrTimeout {
		t.Fatalf("got %v", err)
	}
}

func TestRunDeniedDirCounted(t *testing.T) {
	u, err := user.Current()
	if err == nil && u.Uid == "0" {
		t.Skip("root bypasses directory permissions")
	}
	root := t.TempDir()
	denied := filepath.Join(root, "nope")
	mustMkdir(t, denied)
	mustWrite(t, filepath.Join(denied, "x.txt"), "invoice")
	if err := os.Chmod(denied, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(denied, 0o755) })
	_, stats, err := Run(context.Background(), Plan{Roots: []string{root}, Keywords: []string{"invoice"}})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Denied == 0 {
		t.Fatalf("expected denied count, stats=%+v", stats)
	}
}

func TestRunMaxFiles(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 10; i++ {
		mustWrite(t, filepath.Join(root, "f"+strconv.Itoa(i)+".txt"), "invoice")
	}
	hits, _, err := Run(context.Background(), Plan{
		Roots:    []string{root},
		Keywords: []string{"invoice"},
		MaxFiles: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 3 {
		t.Fatalf("got %d", len(hits))
	}
}

func mustMkdir(t *testing.T, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFixtureTreeSkipsSecrets(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	tree := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "tree")
	hits, stats, err := Run(context.Background(), Plan{
		Roots:    []string{tree},
		Keywords: []string{"invoice"},
		MaxFiles: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected invoice hits, stats=%+v", stats)
	}
	var sawNotes bool
	for _, h := range hits {
		if IsSecretPath(h.Path) {
			t.Fatalf("secret hit %s", h.Path)
		}
		if strings.Contains(h.Path, "id_rsa") || strings.Contains(h.Path, ".env") {
			t.Fatalf("secret leaked %s", h.Path)
		}
		if filepath.Base(h.Path) == "invoice-notes.txt" {
			sawNotes = true
		}
	}
	if !sawNotes {
		t.Fatalf("missing invoice-notes.txt in %v", paths(hits))
	}
	if stats.SecretSkip == 0 {
		t.Fatalf("expected secret skips, stats=%+v", stats)
	}
}

func paths(hits []Hit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.Path
	}
	return out
}
