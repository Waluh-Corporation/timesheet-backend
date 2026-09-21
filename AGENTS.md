# AI Agent Guidelines & Operating Instructions

This repository is **Timesheet Generator Backend** (`timesheet-backend`), a high-performance multi-company timesheet generation API and engine built with **Go (Gin Framework)** and **PostgreSQL 16**.

All AI agents (Antigravity, Copilot, Cursor, etc.) working on this repository MUST strictly follow the directives outlined in this document.

---

## 1. Git Workflow & Branching Discipline

Before creating branches, writing commits, or opening PRs, review [`docs/GIT_WORKFLOW.md`](docs/GIT_WORKFLOW.md).

1. **Base Branch Rule**:
   - Every new feature or bugfix branch **MUST** be branched from the latest `main`:
     ```bash
     git checkout main && git pull origin main
     git checkout -b feat/<feature-name>
     ```
   - **Never** branch a new feature from `development`.
2. **PR Target Rule**:
   - Pull Requests for all new work **MUST** target `development`.
   - Only release PRs (`development` → `main`) or critical hotfixes target `main`.
3. **Commit & PR Conventions**:
   - Commit messages and PR titles must strictly follow [Conventional Commits](https://www.conventionalcommits.org/):
     ```
     feat(scope): short description in lowercase
     fix(scope): short description in lowercase
     refactor(scope): architectural improvements
     test(scope): unit/integration test additions
     chore(scope): build, CI/CD, or dependencies
     ```
   - Commits must be atomic. Do not bundle unrelated changes, refactorings, or formatting tweaks into a single commit.
4. **Changelog Maintenance**:
   - Every user-facing feature, fix, or deprecation must update [`CHANGELOG.md`](CHANGELOG.md) under `## [Unreleased]` following [Keep a Changelog](https://keepachangelog.com/).

---

## 2. Architecture & Code Organization

The repository follows a clean, modular Go architecture:

```
├── cmd/             # Administrative commands, seeders, and CLI entry points
├── config/          # Environment configuration loaded via godotenv / os.Getenv
├── database/        # Database connection, GORM setup, and embedded SQL migrations
│   └── migrations/  # Versioned numeric SQL migrations (XXXXXX_name.up.sql / .down.sql)
├── dto/             # Data Transfer Objects
│   ├── request/     # Incoming request payloads with Gin binding tags
│   └── response/    # Outgoing JSON response envelopes and view models
├── handlers/        # Gin controllers and HTTP route handlers
├── mailer/          # Transactional email engine (gomail)
├── models/          # GORM database entities and domain models
├── push/            # WebPush VAPID notification dispatch service
├── scheduler/       # Background cron jobs (Asia/Jakarta timezone)
├── services/        # Excel builders (Excelize), PDF conversion, external APIs
└── docs/            # Swagger specs, architecture documentation, and workflow guides
```

### Architectural Rules:
- **Never expose raw database models directly to API clients**:
  - Always map `models.*` entities to structured DTOs in `dto/response`.
  - Always validate incoming client input using `dto/request` structs with Gin binding tags.
- **Maintain 3NF Database Integrity**:
  - Database schema alterations must **never** rely on GORM AutoMigrate in production.
  - Any schema change **MUST** include both `.up.sql` and matching `.down.sql` scripts under `database/migrations/` using the next sequential integer (e.g., `000032_...`).
- **Data Ownership & IDOR Protection**:
  - Always enforce user ownership (`user_id = ?`) and role boundaries (`admin` vs `user`) on every query and mutation.
- **Timezone Awareness**:
  - Application logic and scheduled tasks operate in Indonesian Western Time (`Asia/Jakarta` / UTC+7). Always respect proper timezone conversions when handling timestamps.

---

## 3. Go Coding Standards & Idiomatic Practices

All Go code written in this repository must adhere to idiomatic Go best practices and the Go Proverbs:

### 3.1. Idiomatic Conventions & Design
- **Accept Interfaces, Return Structs**: Functions and constructors should accept interface parameters and return concrete types (`*MyStruct`). Keep interfaces small, focused, and defined where they are consumed.
- **Package & Identifier Naming**:
  - Avoid package stuttering (e.g., use `user.Service` instead of `user.UserService`, `holiday.Client` instead of `holiday.HolidayClient`).
  - Use camelCase for unexported identifiers and PascalCase for exported identifiers. Avoid snake_case in Go identifiers.
  - Variable names should be short and proportional to their scope length (e.g., `db` or `u` in small functions, descriptive names in larger scopes).
- **Constructors & Options**:
  - Use `New<StructName>` constructors returning `(*StructName, error)` or `*StructName`.
  - Use functional options (`type Option func(*Options)`) or explicit config structs for complex initialization.
- **Exported Documentation**:
  - Every exported function, struct, interface, and constant MUST have a Godoc comment starting with the item's name (e.g., `// GenerateTimesheet creates an Excel workbook...`).

### 3.2. Error Handling Excellence
- **Explicit Error Values**: Never ignore returned errors (`_ = fn()`). Always handle errors explicitly using `if err != nil`.
- **Error Wrapping with Context**: Wrap lower-level errors with operational context using `fmt.Errorf("failed to fetch user %d: %w", userID, err)` to preserve the causal error chain.
- **Error Inspection**: Always inspect wrapped errors using `errors.Is(err, ErrTarget)` and `errors.As(err, &targetErr)`. Never use string comparison (`err.Error() == "..."`).
- **Sentinel & Domain Errors**: Define package-level sentinel errors (`var ErrNotFound = errors.New(...)`) for predictable error conditions.
- **No Panics**: Never call `panic()` or `log.Fatal()` in HTTP handlers, middleware, services, or background tasks. Use `panic()` exclusively during application bootstrap in `main.go` for fatal, unrecoverable misconfigurations.

### 3.3. Context & Concurrency Discipline
- **Context Propagation**:
  - Every blocking, I/O-bound, or database operation MUST accept `context.Context` as its first parameter (`ctx context.Context`).
  - Respect cancellation and timeouts (`ctx.Done()`, `ctx.Err()`). Never swallow `ctx.Err()`.
- **Goroutine Lifecycle Management**:
  - Never start a goroutine without a deterministic lifecycle. Every background task must be terminable via context cancellation or bounded by `sync.WaitGroup` / worker pools.
  - Guard against goroutine leaks and race conditions. All concurrent access to shared mutable state must be synchronized using `sync.Mutex` or `sync.RWMutex`.
  - Validate with `-race` in all automated tests.

### 3.4. Performance & Memory Optimization
- **Slice & Map Pre-allocation**: Always pre-allocate capacity when the size is known:
  ```go
  // Good: pre-allocated capacity
  activities := make([]dto.ActivityResponse, 0, len(models))
  // Bad: repeatedly triggers memory reallocation and copying
  var activities []dto.ActivityResponse
  ```
- **Efficient String Building**: Use `strings.Builder` or `bytes.Buffer` for dynamic string concatenation in loops instead of `+` or `+=`.
- **Pointer vs. Value Semantics**:
  - Use pointer receivers (`func (s *Service) ...`) for structs that have mutable state or large memory footprints.
  - Use value semantics for small, immutable data structures and pure value types.

### 3.5. Testing Standards (Table-Driven Tests)
- **Table-Driven Pattern**: Write unit tests using table-driven tests organized with `t.Run(tc.name, func(t *testing.T) { ... })`.
- **Comprehensive Scenarios**: Cover happy paths, negative error branches, boundary cases, and invalid inputs.
- **Mocking via Interfaces**: Decouple external network dependencies, mailers, and third-party APIs using Go interfaces to facilitate isolated unit testing.

---

## 4. Pre-Commit Verification & Quality Gates

Before marking any task as complete or asking the user to review, you **MUST** run and pass the following quality gates locally:

```bash
# 1. Code Formatting (mandatory in CI)
gofmt -l .
# If any unformatted files are found, format them:
gofmt -w .

# 2. Static Analysis & Linting
go vet ./...
golangci-lint run

# 3. Unit & Integration Tests (with Race Detector)
go test -v -race ./...

# 4. Git Security & Secrets Check
git status
git diff --staged
```

### Non-Negotiable Test Rules:
- **Never bypass, delete, or comment out failing tests** to force a green build. Diagnose the root cause and fix the code or update the test assertions to reflect intended business logic changes.
- **Always accompany new endpoints or changed handler logic with unit tests** under the respective package (`*_test.go`).

---

## 5. Operational Safety & Behavioral Constraints

1. **Scope Discipline**:
   - Modify only the files strictly necessary to accomplish the user's objective.
   - Do not perform unsolicited refactorings, rename public APIs, or rewrite unrelated files.
2. **Preserve Documentation & Comments**:
   - Retain existing code comments, docstrings, Swagger annotations (`@Summary`, `@Tags`, `@Param`), and architectural notes.
3. **No Hardcoded Credentials**:
   - Never commit API keys, database passwords, JWT secrets, or SMTP tokens. Always read from environment variables via the `config` package.
4. **Environment File Guard**:
   - Never edit or commit `.env` with real credentials. Document new configuration options in `.env.example`.
