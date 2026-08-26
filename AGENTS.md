# AGENTS.md

## Project overview

`gq` is a small Go command-line AI agent. It sends prompts to OpenRouter, displays the model's response as terminal Markdown, and allows the model to request shell commands through a `cmd` tool. Every requested command is shown to the user and requires explicit interactive approval before execution.

The application also supports:

- Continuing the most recent conversation with `gq c` or `gq continue`.
- Persisting the latest conversation at `~/.gq/sessions/last.jsonl`.
- Layered project and user configuration through `.gq/config.json` and context files.
- `gq version` and `gq model` subcommands.

## Repository layout

- `main.go` — application entry point.
- `cmd/` — Cobra commands, OpenRouter API integration, conversation/tool loop, session persistence, terminal rendering, and shell execution.
- `config/` — discovery and merging of configuration files and project context.
- `build.go` — ignored helper for building or cleaning a local binary.
- `PKGBUILD` — Arch Linux package definition.

## Development commands

Use the Go toolchain from the repository root:

```bash
go build ./...
go test ./...
go vet ./...
go run . version
go run . --help
```

The repository currently has no test files, so add focused tests when changing configuration merging, session repair/persistence, command approval, or API response handling.

The optional build helper can be used with:

```bash
go run build.go build
go run build.go clean
```

## Configuration and runtime requirements

- The program obtains its OpenRouter API key from `pass show pi/openrouter`; do not hard-code, log, or commit credentials.
- The configured API endpoint is `https://openrouter.ai/api/v1/chat/completions`.
- The current request model is defined in `cmd/cmd.go`; the `model` command is currently informational and does not persist a model.
- Configuration is JSON. Application defaults are loaded first, followed by `~/.gq/config.json`, then discovered project `.gq/config.json` files from parent to child. `keepWalking` controls upward discovery and `contextFiles` adds file contents to the system context.
- Session files are private (`0600`) and should remain outside version control.

## Change guidance

- Preserve the explicit approval gate for all model-generated shell commands. Never execute tool requests silently.
- Keep shell commands simple and easy for the user to read and approve. Prefer one singular command at a time over chained commands, pipelines, loops, subshells, or dense command substitutions. Very complex or difficult-to-review commands should be rejected and replaced with smaller steps.
- Before making a shell/tool call, briefly explain what will be executed and why, so the user can understand the action before approving it. Do not use the explanation as a substitute for the explicit approval gate.
- Treat model output, tool arguments, API responses, and configuration files as untrusted input. Validate JSON and handle non-success HTTP responses.
- Keep command execution behavior obvious: commands currently run with `bash -c` in the current working directory and capture combined stdout/stderr.
- Maintain session consistency for assistant tool calls and matching tool results, including when resuming an interrupted session.
- Keep user-facing output terminal-friendly and preserve Markdown rendering behavior.
- Prefer small, idiomatic Go changes. Return wrapped errors with useful context and avoid ignoring errors unless there is a deliberate reason.
- Do not commit generated binaries, package archives, credentials, home-directory session data, or unrelated formatting changes.

## Verification checklist

Before submitting a change:

1. Run `gofmt` on modified Go files.
2. Run `go build ./...`, `go test ./...`, and `go vet ./...`.
3. Exercise the relevant CLI help/version path.
4. For API, session, or tool-loop changes, test failure paths as well as the successful path; do not use real credentials in tests.
5. Review `git diff` and `git status` for accidental generated or sensitive files.
