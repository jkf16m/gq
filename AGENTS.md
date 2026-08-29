# AGENTS.md

## Architecture

Every module is composed of two parts or more:

1. **Public API** — minimal, contract only. No implementation.
2. **Private implementation** — can be divided into several files.

We start from private implementation, writing tests to check if they're good, until they're all working as expected. Then we move into the public API. Public API is no longer first approach, it's the end result.

## Build

Use `build.go` from the repository root:

```bash
go run build.go install  # install gq to $GOPATH/bin/
go run build.go build    # build gq binary to bin/
go run build.go clean    # remove bin/
```

## Tests

Integration tests live in `gq/llm_integration_test.go` and require a real API key from `pass show pi/openrouter`:

```bash
go run build.go test
```

## Requirements

- API key: `pass show pi/openrouter` — never hard-code or commit credentials.
- Endpoint: `https://openrouter.ai/api/v1/chat/completions`
- Do not commit generated binaries, credentials, or session data.

## Sessions

Sessions are stored as JSON files in `~/.gq/sessions/` by default. Users can customize the storage path via `.gq/config.json`:

```json
{
  "keepWalking": false,
  "contextFiles": ["AGENTS.md"],
  "storagePath": "/custom/path/sessions"
}
```

Session files are private (`0600`) and should remain outside version control.
