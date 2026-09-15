# Security policy

## What this tool does

`easy` walks directories **read-only** as your user, then may send **truncated file snippets** to a cloud LLM (Gemini, OpenAI, or Anthropic) to plan the search and rank matches.

The skip-list is **best-effort**, not a guarantee. Do not point this tool at trees that contain production secrets, customer data, or other people’s files.

## What we never want in Git or issues

- API keys (`GEMINI_API_KEY`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`)
- Real `.env` files, PEM keys, kube configs, cloud credentials
- The local `.tools/` Go toolchain or built `easy` binaries

If a key was ever exported in a shell or pasted into chat, **rotate it** at the provider before sharing the repo.

## Reporting a vulnerability

Email the maintainer listed in the GitHub profile for this repository. Please include:

- What leaked or what an attacker could do
- A minimal reproduction that does **not** include real secrets

We will not run live paid-API tests in CI and we will not accept patches that weaken the secret skip-list or the root/sudo refusal without a strong reason.

## Hard rules in the product

- Refuses `sudo` and (by default) euid 0. `EASY_ALLOW_ROOT=1` is unsupported.
- Default search scope is `$HOME`, not `/`.
- Snippets are not sent until you confirm (or pass `--yes` / `EASY_YES=1`).
- Directory symlinks are not followed.
- The only writes are under XDG (`~/.config/easy-cli`), never into the search tree.
