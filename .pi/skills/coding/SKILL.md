---
name: coding
description: |
  Coding skill that prioritizes defining interfaces first, with implementations only
  when explicitly requested. Uses hardcoded values to make code compile.
---

# Coding Skill

## Priority

When coding, prioritize defining the interfaces (types, structs, interfaces, function signatures) before anything else.

## Rules

1. **Define interfaces first** — types, structs, interfaces, function signatures.
2. **Do not implement** unless the user explicitly asks for it to be implemented.
3. **Make code compile** — use hardcoded values where needed so the code compiles.
4. **Keep implementations minimal** — only implement what's requested.
5. **Document decisions** — briefly note why a hardcoded value is used.

## Examples

- Define a `Session` struct with fields, but don't implement `Save()` unless asked.
- Use `const defaultSessionDir = "/tmp/sessions"` to make code compile.
- Only add `func SaveSession(...)` when the user says "implement session saving".

## When to apply

Use this skill when writing or modifying Go code, especially when:
- Adding new types or structs
- Creating function signatures
- Refactoring existing code
- Building out new features