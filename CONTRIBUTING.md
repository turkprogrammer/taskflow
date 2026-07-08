# Contributing

## Prerequisites
- Go 1.26+
- Docker and Docker Compose (for MySQL + Redis in integration tests)

## Getting Started

1. Clone the repository
2. Copy config.example.yaml to config.yaml and adjust settings
3. Start dependencies: `docker-compose up -d`
4. Run the server: `go run ./cmd/api`
5. Run tests: `go test ./...`

## Project Layout

- `cmd/api/` — Application entry point
- `internal/config/` — Configuration loading
- `internal/handler/` — HTTP handlers and middleware
- `internal/lib/` — Shared utilities (JWT, circuit breaker)
- `internal/migrate/` — Database migration runner
- `internal/service/` — Business logic layer
- `internal/store/` — Data access layer (MySQL + Redis)
- `cmd/api/migrations/` — SQL migration files

## Branching and PRs

- Create a feature branch from main
- Keep changes focused and atomic
- Write tests for new functionality
- Ensure all tests pass before opening a PR
- Update documentation if changing public API

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Use `log/slog` for structured logging
- Use custom `database/sql` — no ORMs
- Every exported symbol must have a godoc comment
- Business logic goes in service layer, not handlers

## Testing

- Unit tests: `go test ./internal/service/...`
- Integration tests: `go test ./internal/store/...` (requires Docker)
- Coverage: `go test -cover ./internal/...`
- Mock generation: uses go.uber.org/mock

## Commit Messages

Write clear, conventional commit messages:
- feat: new feature
- fix: bug fix
- docs: documentation changes
- test: test additions or changes
- refactor: code restructuring
