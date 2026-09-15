package repl

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpAndExit(t *testing.T) {
	in := strings.NewReader("help\nexit\n")
	var out bytes.Buffer
	if err := Run(IO{In: in, Out: &out}); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "find") || !strings.Contains(s, "easy>") {
		t.Fatalf("%s", s)
	}
}

func TestScopePersists(t *testing.T) {
	in := strings.NewReader("scope /tmp /var/log\nscope\nexit\n")
	var out bytes.Buffer
	if err := Run(IO{In: in, Out: &out}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "scope") || !strings.Contains(out.String(), "/tmp") {
		t.Fatalf("%s", out.String())
	}
}

func TestIsQuestion(t *testing.T) {
	if !isQuestion("what does the first one say?") {
		t.Fatal("expected question")
	}
	if isQuestion("invoices from last month") {
		t.Fatal("expected search phrase")
	}
}
