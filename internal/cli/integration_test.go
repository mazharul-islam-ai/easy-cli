//go:build integration

package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBinarySearchFixture(t *testing.T) {
	root := moduleRoot(t)
	tree := filepath.Join(root, "testdata", "tree")
	bin := buildEasy(t, root)
	srv := mockOpenAI(t, tree)
	t.Cleanup(srv.Close)

	out, err := runEasy(t, root, bin, srv.URL+"/v1", "search", "--scope", tree, "PDFs about invoices from last month")
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "invoice") {
		t.Fatalf("expected invoice in output:\n%s", out)
	}
	if strings.Contains(out, "id_rsa") || strings.Contains(out, "SECRET=") || strings.Contains(out, ".env") {
		t.Fatalf("secret leaked:\n%s", out)
	}
}

func TestBinaryREPLSmoke(t *testing.T) {
	root := moduleRoot(t)
	tree := filepath.Join(root, "testdata", "tree")
	bin := buildEasy(t, root)
	srv := mockOpenAI(t, tree)
	t.Cleanup(srv.Close)

	script := "scope " + tree + "\nfind invoices from last month\nwhat does the first one say about payment terms?\nexit\n"
	cmd := easyCmd(t, root, bin, srv.URL+"/v1")
	cmd.Stdin = strings.NewReader(script)
	body, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("repl: %v\n%s", err, body)
	}
	out := string(body)
	if !strings.Contains(out, "invoice") {
		t.Fatalf("expected invoice:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "net 30") {
		t.Fatalf("expected follow-up answer:\n%s", out)
	}
	if strings.Contains(out, "id_rsa") || strings.Contains(out, "SECRET=") {
		t.Fatalf("secret leaked:\n%s", out)
	}
}

func mockOpenAI(t *testing.T, tree string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s := string(body)
		content := "Payment terms are net 30."
		switch {
		case strings.Contains(s, "Question:"):
			content = "Payment terms are net 30."
		case strings.Contains(s, "SearchPlan"):
			content = `{"keywords":["invoice"],"extensions":["txt","pdf","md"],"name_globs":[],"max_files":50}`
		case strings.Contains(s, "PATH:"):
			var items []map[string]any
			_ = filepath.Walk(tree, func(path string, info os.FileInfo, err error) error {
				if err != nil || info == nil || info.IsDir() {
					return nil
				}
				if strings.Contains(strings.ToLower(path), "invoice") && strings.Contains(s, path) {
					items = append(items, map[string]any{
						"path":   path,
						"score":  0.9,
						"reason": "invoice document",
					})
				}
				return nil
			})
			raw, _ := json.Marshal(map[string]any{"items": items, "summary": "Found invoice documents"})
			content = string(raw)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-1",
			"object":  "chat.completion",
			"created": 1,
			"model":   "gpt-4.1-mini",
			"choices": []map[string]any{{
				"index":         0,
				"finish_reason": "stop",
				"message":       map[string]any{"role": "assistant", "content": content},
			}},
		})
	}))
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func buildEasy(t *testing.T, root string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "easy")
	cmd := exec.Command(goBin(root), "build", "-o", bin, "./cmd/easy")
	cmd.Dir = root
	cmd.Env = goEnv(root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

func runEasy(t *testing.T, root, bin, baseURL string, args ...string) (string, error) {
	t.Helper()
	cmd := easyCmd(t, root, bin, baseURL)
	cmd.Args = append(cmd.Args, args...)
	body, err := cmd.CombinedOutput()
	return string(body), err
}

func easyCmd(t *testing.T, root, bin, baseURL string) *exec.Cmd {
	t.Helper()
	cfgHome := t.TempDir()
	cmd := exec.Command(bin)
	cmd.Dir = root
	cmd.Env = []string{
		"HOME=" + t.TempDir(),
		"XDG_CONFIG_HOME=" + cfgHome,
		"EASY_PROVIDER=openai",
		"OPENAI_API_KEY=sk-test",
		"OPENAI_BASE_URL=" + baseURL,
		"PATH=" + os.Getenv("PATH"),
		"TERM=dumb",
		"EASY_ALLOW_ROOT=1",
		"EASY_YES=1",
	}
	return cmd
}

func goBin(root string) string {
	p := filepath.Join(root, ".tools", "go", "bin", "go")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return "go"
}

func goEnv(root string) []string {
	env := os.Environ()
	goroot := filepath.Join(root, ".tools", "go")
	if _, err := os.Stat(goroot); err == nil {
		env = append(env,
			"GOROOT="+goroot,
			"GOTOOLCHAIN=local",
			"PATH="+filepath.Join(goroot, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
			"GOMODCACHE="+filepath.Join(root, ".tools", "pkg", "mod"),
			"GOCACHE="+filepath.Join(root, ".tools", "gocache"),
			"GOPATH="+filepath.Join(root, ".tools", "gopath"),
		)
	}
	return env
}
