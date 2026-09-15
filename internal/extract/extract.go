// Package extract reads file contents read-only and truncates snippets for the LLM.
package extract

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"easy-cli/internal/search"
)

const defaultLimit = 4096

// Snippet is truncated text from a file.
type Snippet struct {
	Path    string
	Text    string
	Trunced bool
	Skipped string
}

// File reads path read-only and returns at most limit bytes of text.
func File(path string, limit int) Snippet {
	if limit <= 0 {
		limit = defaultLimit
	}
	s := Snippet{Path: path}
	if search.IsSecretPath(path) {
		s.Skipped = "secret path"
		return s
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".mp4", ".mov", ".zip", ".gz", ".bz2", ".xz", ".7z", ".exe", ".so", ".o", ".bin":
		s.Skipped = "binary type"
		return s
	}

	if ext == ".pdf" {
		if text, ok := pdfText(path, limit); ok {
			s.Text, s.Trunced = truncate(text, limit)
			return s
		}
	}

	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		s.Skipped = "unreadable"
		return s
	}
	defer f.Close()
	buf := make([]byte, limit+1)
	n, _ := f.Read(buf)
	buf = buf[:n]
	if isBinary(buf) && ext != ".pdf" {
		s.Skipped = "binary content"
		return s
	}
	text := string(bytes.Trim(buf, "\x00"))
	if !utf8.ValidString(text) {
		text = strings.ToValidUTF8(text, "")
	}
	s.Text, s.Trunced = truncate(text, limit)
	return s
}

func truncate(text string, limit int) (string, bool) {
	if len(text) <= limit {
		return text, false
	}
	// Avoid splitting a rune.
	cut := limit
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut], true
}

func isBinary(b []byte) bool {
	n := len(b)
	if n > 800 {
		n = 800
	}
	for i := 0; i < n; i++ {
		if b[i] == 0 {
			return true
		}
	}
	return false
}

func pdfText(path string, limit int) (string, bool) {
	if _, err := exec.LookPath("pdftotext"); err == nil {
		cmd := exec.Command("pdftotext", "-f", "1", "-l", "3", "-q", path, "-")
		out, err := cmd.Output()
		if err == nil && len(bytes.TrimSpace(out)) > 0 {
			s, _ := truncate(string(out), limit)
			return s, true
		}
	}
	return pdfRawStrings(path, limit)
}

func pdfRawStrings(path string, limit int) (string, bool) {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return "", false
	}
	defer f.Close()
	buf := make([]byte, 256*1024)
	n, _ := f.Read(buf)
	buf = buf[:n]
	if !bytes.Contains(buf, []byte("%PDF")) {
		return "", false
	}
	var b strings.Builder
	for i := 0; i < len(buf); i++ {
		if buf[i] == '(' {
			j := i + 1
			for j < len(buf) && buf[j] != ')' {
				if buf[j] == '\\' && j+1 < len(buf) {
					j += 2
					continue
				}
				j++
			}
			if j < len(buf) && j-i > 2 {
				chunk := buf[i+1 : j]
				if utf8.Valid(chunk) && looksLikeText(chunk) {
					b.Write(chunk)
					b.WriteByte(' ')
				}
			}
			i = j
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "", false
	}
	s, _ := truncate(out, limit)
	return s, true
}

func looksLikeText(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	letters := 0
	for _, c := range b {
		if c >= 0x20 && c < 0x7f {
			letters++
		} else if c == '\n' || c == '\r' || c == '\t' {
			continue
		} else {
			return false
		}
	}
	return letters > 0
}
