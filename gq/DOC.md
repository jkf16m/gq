# gq

A minimal CLI agent that executes bash commands through an LLM.

## What it does

gq is a terminal-based AI agent. You type a prompt, the LLM responds, and when it needs to run a command, it asks for your permission.

## How it works

1. User types a prompt
2. gq loads context from `.gq/` folders and files
3. LLM processes the prompt with context
4. If LLM wants to run a command:
   - Shows the command: `$ ls -la`
   - User presses `y` twice to accept, `n` to reject
   - If accepted: execute command, show output, continue loop
   - If rejected: agent responds to user, loop ends
5. If LLM responds with text: show it, done

## Current limitations

- **TTY mode only** — no stdin piping, no args mode
- **In-memory sessions** — no file persistence yet
- **Single tool** — only `cmd` for bash execution
- **Hardcoded API key** — reads from `pass show pi/openrouter`
- **Hardcoded model** — uses `@preset/mimo`

## Configuration

gq looks for `.gq/config.json` in the current directory and walks up the tree.

```json
{
  "keepWalking": true,
  "include": ["*.go", "*.md"],
  "exclude": ["vendor/**", "*.exe"],
  "useGitignore": true
}
```

### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `keepWalking` | bool | `false` | Continue walking to parent directories |
| `include` | string[] | `["*"]` | Glob patterns for files to include |
| `exclude` | string[] | `[]` | Glob patterns for files to exclude |
| `useGitignore` | bool | `false` | Use `.gitignore` rules to exclude files |

### Walking behavior

1. Start in current directory
2. Look for `.gq/config.json`
3. If found:
   - Load config
   - Read files based on include/exclude rules
   - If `keepWalking: true` → continue to parent directory
   - If `keepWalking: false` or missing → stop walking
4. If not found → stop walking

### Example

```
project/
├── .gq/
│   └── config.json    # keepWalking: true, include: ["*.go"]
├── src/
│   ├── .gq/
│   │   └── config.json  # keepWalking: false (default)
│   └── main.go
└── README.md
```

Running `gq` in `src/`:
- Loads `src/.gq/config.json` (keepWalking: false → stops)
- Reads `src/main.go` (matches include pattern)
- Does NOT read `README.md` (stopped walking)

Running `gq` in project root:
- Loads `.gq/config.json` (keepWalking: true → continues)
- No `.gq/config.json` in parent → stops
- Reads files matching include patterns

## Context loading

1. Walk directories looking for `.gq/` folders
2. Load config from each `.gq/config.json`
3. Collect files based on include/exclude rules
4. Sort files by modification time (oldest first)
5. Inject file contents as context to the LLM

## Agent loop

```
User: "list files in this directory"
  ↓
LLM: [tool_call: cmd, command: "ls -la"]
  ↓
gq: $ ls -la
gq: y/n: y
gq: y/n: y
  ↓
[execute command, show output]
  ↓
LLM: "Here are the files: ..."
  ↓
gq: [show response]
```

## File structure

Single file: `gq.go`

- Types (Message, ToolCall, ChatRequest, etc.)
- LLM client (API key, HTTP requests)
- Agent loop (tool call handling, confirmation flow)
- Context loading (file reading, directory walking)
- Main (TTY loop)
