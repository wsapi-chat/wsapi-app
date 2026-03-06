# Contributing to WSAPI

Thank you for your interest in contributing! This guide will help you get started.

## Development Setup

### Prerequisites

- **Go 1.25+** — [install](https://go.dev/doc/install)
- **golangci-lint** (optional) — `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`

### Building

```bash
git clone https://github.com/wsapi-chat/wsapi-app.git
cd wsapi-app
make build
```

### Running

```bash
cp config.example.yaml config.yaml
make build && ./bin/server
```

Or with Docker:

```bash
docker compose up --build
```

## Workflow

1. **Fork** the repository
2. **Create a branch** from `main` (`git checkout -b feat/my-feature`)
3. **Make your changes** — follow the coding standards below
4. **Test** — `make test`
5. **Lint** — `make lint`
6. **Commit** — use [Conventional Commits](https://www.conventionalcommits.org/) format
7. **Push** and open a **Pull Request** against `main`

## Coding Standards

### Project Structure

WSAPI follows a three-layer architecture:

```
Handler (request parsing, validation, response) ->
  WhatsApp Service (business logic, whatsmeow calls) ->
    whatsmeow Client (protocol layer)
```

### Key Patterns

- **Handler composition** — all handlers embed the `Handler` base struct for shared helpers (`Instance()`, `Decode()`, `JSON()`, `ServiceError()`, `Error()`)
- **Service composition** — `whatsapp.Service` exposes domain sub-services (`Messages`, `Groups`, `Communities`, etc.)
- **Projector pattern** — each event domain has its own `projector_*.go` file; `projector.go` dispatches via type switch
- **Publisher factory** — `Factory.Create()` selects the publisher based on config

### Adding Features

| Task | Files to Edit |
|------|---------------|
| New API endpoint | `internal/handler/<domain>.go` -> `internal/whatsapp/<domain>.go` -> `cmd/server/main.go` (register route) |
| New event type | `internal/event/constants.go` -> `internal/event/projector_<domain>.go` -> `internal/event/types.go` -> `internal/event/projector.go` |
| Configuration | `internal/config/config.go` -> `config.example.yaml` -> `.env.example` |

See [`.claude/development-guide.md`](.claude/development-guide.md) for detailed examples.

### Database Migrations

- Append a new `waMigrateVN` function in `internal/whatsapp/migrate.go` and add it to the `allWAMigrations` slice

Never modify existing migrations.

## Commit Conventions

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add newsletter unsubscribe endpoint
fix: handle nil pointer in group projector
docs: update OpenAPI spec for status endpoints
refactor: extract media encoding to shared helper
```

## Testing

```bash
make test           # run all tests
make test-coverage  # run with coverage report
```

Tests use the Go standard library only. Place test files alongside the code they test (`*_test.go`).

## OpenAPI Specs

If your change adds, modifies, or removes an API endpoint or event type, update the corresponding OpenAPI spec:

- `openapi/wsapi-api.yml` — REST API endpoints
- `openapi/wsapi-events.yml` — event payload schemas

Validate with:

```bash
make openapi-lint
```

## `.claude/` Directory

The `.claude/` directory contains detailed architecture documentation used by AI coding assistants. These files are maintained alongside the codebase — if your change affects the architecture, please update the relevant docs.

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## Questions?

Open a [discussion](https://github.com/wsapi-chat/wsapi-app/discussions) or [issue](https://github.com/wsapi-chat/wsapi-app/issues).
