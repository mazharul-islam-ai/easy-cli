# easy-cli

Ubuntu-first interactive CLI that turns a natural-language request into a **local, read-only** filesystem search, then uses **Gemini, ChatGPT, or Claude** only to plan the query and rank a small snippet pack.

The model never walks the disk. Default search root is **`$HOME`** as the current user. The process **refuses sudo** and, by default, **refuses root**.

**Privacy:** truncated file contents are sent to a cloud provider only after you confirm (or pass `--yes`). The skip-list is best-effort — do not search trees that contain production secrets.

## Requirements

- Ubuntu 22.04/24.04
- Go 1.24+ to build (`go version`)
- An API key for at least one of: Gemini (`GEMINI_API_KEY`), OpenAI (`OPENAI_API_KEY`), Anthropic (`ANTHROPIC_API_KEY`)
- Optional: `poppler-utils` (`pdftotext`) for better PDF text

## Install

From source:

```bash
git clone https://github.com/mazharul-islam-ai/easy-cli.git
cd easy-cli
make build
sudo install -m 0755 easy /usr/local/bin/easy
```

After the GitHub repository exists, tagged releases will attach Linux amd64/arm64 tarballs plus `checksums.txt`. Change the module path in `go.mod` to `github.com/<you>/easy-cli` if the clone URL differs, then:

```bash
go install github.com/<you>/easy-cli/cmd/easy@v1.0.0
```

## Run

```bash
export GEMINI_API_KEY=...          # or OPENAI_API_KEY / ANTHROPIC_API_KEY
easy                               # TUI (default scope $HOME)
easy --scope "$HOME/Documents"
easy search "PDFs about invoices from last month"
easy search --yes --scope "$HOME/Documents" "contracts mentioning termination"
easy config
easy config init
easy version
```

Inside the TUI, type a query and press enter. Status shows planning, walking, and ranking. Before snippets leave the machine you get a file list — `y` sends them, `n` keeps local filename matches. After a confirmed search, questions like `what does the first one say?` go to the model.

```text
find PDFs about invoices from last month
scope /home/you/Documents
scope /                 # whole disk as your user — not sudo
help
```

Ctrl+C during a search **cancels** it. Ctrl+C at the idle prompt quits.

Non-interactive sessions (pipes, CI) must pass `--yes` or `EASY_YES=1` or they will refuse to send snippets.

## Configuration

Keys stay in the **environment**. Optional XDG file: `~/.config/easy-cli/config.toml` (`easy config init`).

```toml
[provider]
name = "gemini"          # openai | anthropic | gemini
model = ""               # empty = provider default

[search]
timeout_seconds = 15
max_files = 200
max_rank = 20
snippet_bytes = 4096

[privacy]
auto_confirm = false
```

Hard caps (values above these are clamped): timeout 300s, 2000 files, 50 ranked hits, 16384 snippet bytes.

| Variable | Meaning |
|---|---|
| `EASY_PROVIDER` | `openai`, `anthropic`, or `gemini` |
| `EASY_MODEL` | model id |
| `EASY_TIMEOUT_SECONDS` | walk timeout |
| `EASY_YES` / `EASY_AUTO_CONFIRM` | skip the send-snippets prompt |
| `GEMINI_API_KEY` / `OPENAI_API_KEY` / `ANTHROPIC_API_KEY` | credentials |
| `GEMINI_BASE_URL` / `OPENAI_BASE_URL` / `ANTHROPIC_BASE_URL` | mock or proxy (tests) |
| `EASY_ALLOW_ROOT` | unsupported; allows euid 0. `sudo` is always refused |

Copy `.env.example` as a cheat-sheet. Do not commit real keys. If a key was ever pasted into a terminal or chat, rotate it.

## What is skipped (never walked, never sent)

- Virtual/system: `/proc`, `/sys`, `/dev`, `/run`, `/snap`, `/lost+found`
- Junk: `/tmp`, `/var/tmp`, `/var/cache`, `.git`, `node_modules`, `.cache`
- Secrets: `~/.ssh`, `~/.gnupg`, `~/.aws`, `~/.kube`, `~/.docker`, `~/.password-store`, `*.pem`, `*.key`, `*.p12`, `id_rsa*`, `.env`, `.netrc`, `.npmrc`, `credentials.json`, `/etc/shadow`
- Directory symlinks are not followed

Permission errors are counted, not dumped. The tool only opens files `O_RDONLY`. The only writes are under XDG (`~/.config/easy-cli`).

## Tests

```bash
make lint       # gofmt + go vet (+ golangci-lint if installed)
make test       # unit tests, offline
make test-int   # integration: build binary, fixture tree, mock LLM HTTP
make build
```

Unit tests never hit live APIs. Integration tests (`go test -tags=integration ./...`) start a mock OpenAI server. Do not put paid keys in CI.

## Layout

```
cmd/easy            # main only
internal/cli        # Cobra
internal/repl       # TUI + line mode
internal/app        # plan → search → extract → confirm → rank
internal/planner    # LLM → SearchPlan JSON
internal/search     # read-only walker + skip lists
internal/extract    # snippets, PDF text
internal/ranker     # LLM rerank + Q&A
internal/provider   # openai / anthropic / gemini + fake
internal/config     # XDG + env
internal/safeguard  # sudo / root refusal
internal/version    # ldflags
testdata/tree       # fixture FS (includes a fake .env that must not leak)
```

## License

MIT. See [LICENSE](LICENSE) and [SECURITY.md](SECURITY.md).
