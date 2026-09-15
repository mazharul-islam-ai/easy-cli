package extract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileTruncates(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "big.txt")
	body := strings.Repeat("a", 8000)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	s := File(p, 100)
	if !s.Trunced {
		t.Fatal("expected truncation")
	}
	if len(s.Text) > 100 {
		t.Fatalf("len=%d", len(s.Text))
	}
}

func TestFileSkipsSecret(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	if err := os.WriteFile(p, []byte("SECRET=1"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := File(p, 4096)
	if s.Skipped == "" || s.Text != "" {
		t.Fatalf("secret should be skipped: %+v", s)
	}
}

func TestFileSkipsBinary(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "blob.bin")
	if err := os.WriteFile(p, []byte{0, 1, 2, 3, 4}, 0o644); err != nil {
		t.Fatal(err)
	}
	s := File(p, 4096)
	if s.Skipped == "" {
		t.Fatalf("expected skip, got %+v", s)
	}
}

func TestFileReadsText(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(p, []byte("hello invoice"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := File(p, 4096)
	if s.Skipped != "" || s.Text != "hello invoice" {
		t.Fatalf("%+v", s)
	}
}

func TestPDFRawStrings(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "invoice.pdf")
	pdf := "%PDF-1.1\nBT (Invoice for last month payment terms net 30) Tj ET\n%%EOF\n"
	if err := os.WriteFile(p, []byte(pdf), 0o644); err != nil {
		t.Fatal(err)
	}
	s := File(p, 4096)
	if !strings.Contains(s.Text, "Invoice for last month") {
		t.Fatalf("pdf text=%q skipped=%q", s.Text, s.Skipped)
	}
}
