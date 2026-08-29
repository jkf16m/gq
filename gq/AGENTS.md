# gq/

CLI entry point and agent orchestration.

## Files

- **main.go** — Cobra CLI setup, mode detection (stdin/args/tty), agent loop runner
- **agent.go** — Agent loop: sends messages to LLM, handles tool calls, manages conversation state
- **cmd.go** — Command execution and user confirmation flow

## Modes

1. **Args** — `gq "prompt"` → single response → exit
2. **Stdin** — `echo "prompt" | gq` → commands queued → exit
3. **TTY** — `gq` → interactive loop with tool confirmation
