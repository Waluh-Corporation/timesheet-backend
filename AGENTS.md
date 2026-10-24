# AI Agent Guidelines & Operating Instructions

Repository: **Timesheet Generator Backend** (`timesheet-backend`) — High-performance multi-company timesheet generation API and engine built with **Go (Gin Framework)** and **PostgreSQL 16**.

All AI agents MUST strictly follow these directives.

---

## 1. Git Workflow & Branching Discipline

Reference: [`docs/GIT_WORKFLOW.md`](docs/GIT_WORKFLOW.md).

- **Base & PR Target Branch**:
  - All new work (`feat/*`, `fix/*`) MUST branch from and target `development`.
  - Only release PRs (`development` → `main`) or critical production hotfixes (`hotfix/*`) target `main`.
- **Merge Strategy**: Always merge PRs using **"Create a merge commit" (`--no-ff`)** to preserve commit traceability.
- **Commit Conventions**: Strictly follow [Conventional Commits](https://www.conventionalcommits.org/): `feat|fix|refactor|test|chore(scope): short description in lowercase`. Keep commits atomic without bundling unrelated changes.
- **Changelog Retention**:
  - Update [`CHANGELOG.md`](CHANGELOG.md) under `## [Unreleased]` following [Keep a Changelog](https://keepachangelog.com/) in clear English.
  - **5-Version Limit**: Strictly maintain only the latest **5 released versions** plus `## [Unreleased]`; prune older entries.

---

## 2. Architecture & Code Organization

- `cmd/`: CLI entry points & seeders | `config/`: Environment config | `database/migrations/`: Sequential SQL migrations (`XXXXXX_name.{up,down}.sql`)
- `dto/request/`: Gin-validated payloads | `dto/response/`: Outgoing API envelopes (never expose raw models)
- `handlers/`: Gin controllers | `models/`: GORM entities (enforce 3NF, no AutoMigrate in prod)
- `services/`: Excelize builders, PDF engines | `scheduler/`: Background cron jobs (TZ-configurable)
- `mailer/`: Gomail engine | `push/`: WebPush VAPID service | `docs/`: Specs & documentation

### Core Architectural Rules:
- **Never expose raw database models**: Always map `models.*` to `dto/response.*` and validate inputs via `dto/request.*`.
- **Database Integrity**: All schema changes require versioned `.up.sql` and matching `.down.sql` migrations.
- **Data Ownership & IDOR Protection**: Always enforce user ownership (`user_id = ?`) and role boundaries (`admin` vs `user`).
- **Timezone Awareness**: Defaults to `UTC`, configurable dynamically via the `TZ` environment variable (with fallback to `SCHEDULER_TZ`) using `config.AppLocation()`. Always respect proper timezone conversions when handling timestamps.

---

## 3. Go Coding Standards & Idiomatic Practices

- **Interface & Naming Design**:
  - Accept interfaces, return concrete structs. Keep interfaces small and defined where consumed.
  - No package stuttering (e.g., `user.Service`, not `user.UserService`).
  - PascalCase for exported identifiers, camelCase for unexported. Avoid snake_case.
  - Exported identifiers must include Godoc comments starting with the identifier's name.
- **Error Handling**:
  - Explicit error handling (`if err != nil`). Never ignore errors (`_ = fn()`).
  - Wrap operational context: `fmt.Errorf("context: %w", err)`. Inspect via `errors.Is` and `errors.As`.
  - Use package-level sentinel errors (`var ErrNotFound = errors.New(...)`).
  - Zero panics or `log.Fatal` in handlers/services/tasks (reserved only for fatal bootstrap in `main.go`).
- **Concurrency & Context**:
  - Propagate `ctx context.Context` as first parameter in blocking/IO operations. Respect cancellation (`ctx.Done()`).
  - Deterministic goroutine lifecycles bounded by `sync.WaitGroup` or worker pools. Protect shared state with `sync.Mutex`/`RWMutex`.
- **Performance & Testing**:
  - Pre-allocate slice and map capacity when length is known. Use `strings.Builder` for concatenation in loops.
  - Pointer receivers for mutable/large structs; value receivers for small immutable types.
  - Table-driven unit tests (`t.Run`) covering happy paths, negative branches, and edge cases. Mock external dependencies via interfaces.

---

## 4. Pre-Commit Verification & Quality Gates

Run and pass all quality gates locally before completing tasks:

```bash
gofmt -w .                 # 1. Code Formatting
go vet ./...               # 2. Static Analysis
golangci-lint run          # 3. Comprehensive Linting
go test -v -race ./...     # 4. Unit & Integration Tests (Race Detector)
git status && git diff     # 5. Cleanliness & Security Check
```

- **Non-Negotiable**: Never bypass, delete, or comment out failing tests. Diagnose and fix root causes.
- Accompany new endpoints or handler changes with unit tests (`*_test.go`).

---

## 5. Operational Safety & Behavioral Constraints

1. **Scope Discipline**: Modify only files strictly necessary for the objective. No unsolicited refactoring.
2. **Preserve Documentation**: Keep existing comments, docstrings, and Swagger annotations (`@Summary`, `@Tags`, `@Param`).
3. **No Hardcoded Credentials**: Read secrets from environment variables via `config`. Never commit credentials.
4. **Environment File Guard**: Never edit or commit `.env`. Document new variables in `.env.example`.
5. **English-Only Messages**: All messages—including commit messages, PR descriptions, code comments, and API response/error messages—MUST strictly be in English.
