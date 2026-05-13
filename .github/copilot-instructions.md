# Copilot instructions for vaults-syncer

## Project overview
- `vaults-syncer` is a Go daemon that synchronizes secrets between multiple vault backends and exposes HTTP endpoints for health, status, and manual sync execution.
- The repository uses standard library HTTP handlers, `log/slog` for structured logging, SQLite for persistence, and `robfig/cron` for scheduled syncs.

## Repository layout
- `main.go` wires configuration, storage, sync engine, scheduler, and HTTP servers together.
- `api/` contains HTTP handlers and their tests.
- `config/` contains configuration types, loading, and validation.
- `sync/` contains the core synchronization engine and scheduler logic.
- `storage/` contains SQLite persistence code.
- `vault/` contains vault client abstractions and implementations.
- `docs/` contains the MkDocs source. Do not edit generated files under `site/`; they are build output and `site/` is gitignored.

## Working conventions
- Keep changes focused and avoid unrelated refactors.
- Follow standard Go formatting with `go fmt ./...` when changing Go files.
- Prefer existing standard library patterns and current project dependencies before adding new packages.
- Keep exported Go APIs documented with comments.
- Preserve structured logging patterns that use `slog` key/value fields.

## Validation
- Baseline local validation is `go build ./...` and `go test ./...`.
- For code changes, also run more targeted package tests when possible before rerunning broader validation.
- If you change documentation tooling or MkDocs content, validate with `mkdocs build`.
- If you change the end-to-end test setup under `e2e/` or related Docker files, validate with `./e2e/test-integration.sh`.

## Documentation and examples
- Update `README.md`, `docs/`, or `examples/config.example.yaml` when behavior or configuration changes.
- Keep configuration examples aligned with the types and validation rules in `config/`.
