# Changelog

## [1.0.0] - unreleased

First production-oriented release.

### Safety

- Default search scope is `$HOME` (use `scope /` or `--scope /` for the whole disk).
- Refuses `sudo` and, by default, running as root (`EASY_ALLOW_ROOT=1` is an unsupported escape hatch).
- Asks before sending file snippets to a cloud model (`--yes` / `EASY_YES=1` for scripts).
- Wider secret skip-list (AWS, kube, Docker config, `.netrc`, PKCS12, …).
- Hard caps on timeout, file count, rank size, and snippet bytes.

### Product

- `easy version` / `--version`
- `easy config init` writes an example config **without** API keys
- TUI: Ctrl+C cancels an in-flight search; a second Ctrl+C quits
