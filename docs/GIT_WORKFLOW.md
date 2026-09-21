# Git Workflow & Versioning Guide

This document outlines the official Git workflow and versioning standards for the **Timesheet Generator Backend** project.
It is specifically designed for a high-velocity team of **2–4 engineers**, balancing **agility**, **production stability**, and **CI/CD automation**.

---

## 1. Branch Architecture & Environments

```
┌────────────────────────────────────────────────────────────────────────┐
│ [Production] branch: main                                              │
│ (Stable release code, protected branch, build triggered by tag: v*.*.*)│
└───────────────────────────┬──────────────────────────────────▲─────────┘
                            │                                  │
                  1. Branch off (Base)                         │ 4. Release PR
                            │                                  │    (Staging -> Prod)
                            ▼                                  │
┌──────────────────────────────────────┐                       │
│ [Work in Progress] feat/* or fix/*   │                       │
│ (Developed locally by engineers)     │                       │
└──────────────────┬───────────────────┘                       │
                   │                                           │
                   │ 2. Pull Request & CI Gates                │
                   ▼                                           │
┌──────────────────────────────────────────────────────────────┴─────────┐
│ [Development / Staging] branch: development                            │
│ (Auto-build Docker image tag: development & dev-<sha>)                 │
│ (Shared integration and staging environment for team verification)     │
└────────────────────────────────────────────────────────────────────────┘
```

### Branch Characteristics

| Branch | Role | Base Branch on Creation | Target PR | CI/CD Triggers |
| :--- | :--- | :--- | :--- | :--- |
| **`main`** | Production-ready, highly stable codebase. | - | Only via Release PR from `development` (or emergency hotfixes) | **Push Tag `v*.*.*`** → Builds & publishes Docker tags `latest` & `v*.*.*`, creates GitHub Release. |
| **`development`** | Shared staging and integration branch. | `main` | From feature (`feat/*`) or bugfix (`fix/*`) branches | **Push/Merge** → Auto-builds Docker tags `development` & `dev-<short-sha>`. |
| **`feat/*` / `fix/*`** | Feature development or bug fixing. | **`main`** | **`development`** | PR triggers Gitleaks, linters, vulnerability audit, unit tests with race detector, and SonarQube. |

---

## 2. Complete Development Lifecycle (Step-by-Step)

### Phase 1: Starting a New Feature or Fix
Every new feature or bugfix must be branched off the latest stable `main`.

```bash
# 1. Switch to main and pull the latest changes
git checkout main
git pull origin main

# 2. Create your working branch following naming conventions
git checkout -b feat/profile-notes
```

#### Branch Naming Conventions:
- `feat/<short-title>`: New features (e.g., `feat/profile-notes`, `feat/export-csv`)
- `fix/<short-title>`: Bug fixes (e.g., `fix/token-expiration`, `fix/tz-offset`)
- `refactor/<short-title>`: Internal code refactoring without behavior change (e.g., `refactor/auth-handler`)
- `chore/<short-title>`: Tooling, CI/CD, or dependency updates (e.g., `chore/update-deps`)

---

### Phase 2: Local Development & Commits (Save Point Pattern)
Adhere to the **Atomic Commit** principle (one logical change per commit) and **Conventional Commits**.

```bash
# Verify modified files and ensure no credentials or .env are staged
git status
git diff

# Run local verification
go test -race ./...
golangci-lint run
gofmt -l .

# Stage and commit with structured message
git add .
git commit -m "feat(profile): add notes field to profile change request dto"
```

#### Commit Message Format:
```
<type>(<scope>): <lowercase short description>

[optional body explaining the 'why', not the 'what']
```
- `feat`: A new feature
- `fix`: A bug fix
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `test`: Adding or correcting tests
- `docs`: Documentation updates only
- `chore`: Tooling, build system, or dependencies

---

### Phase 3: Opening a Pull Request (PR) to `development`

1. **Push your branch to GitHub**:
   ```bash
   git push -u origin feat/profile-notes
   ```

2. **Open PR in GitHub**:
   - **Base branch**: `development`
   - **Compare branch**: `feat/profile-notes`
   - **Title**: Follows Conventional Commits (e.g., `feat(profile): add notes to profile request`), validated by `semantic-pr.yml`.
   - **Description**: Provide context on changes, intentionally untouched areas, and testing steps.

3. **Automated CI Quality Gates**:
   Every PR must pass automated CI checks prior to merging:
   - **Secret Scan (Gitleaks)**: Scans diffs for leaked passwords, tokens, or private keys.
   - **Lint & Security Audit**: Runs `gofmt`, `go vet`, `govulncheck`, and `golangci-lint`.
   - **Automated Tests & Coverage**: Spins up an ephemeral PostgreSQL 16 service container, executes database migrations, and runs all test suites with race detection (`-race`).
   - **SonarQube Quality Gate**: Assesses code duplication, security hotspots, and code smells.
   - **Semantic PR Title**: Validates that PR title adheres to Conventional Commits.

4. **Peer Review**:
   - Requires at least **1 approval** from another team member.
   - Once CI is green and approved, perform **Merge pull request** into `development`.

---

### Phase 4: Staging Verification in `development`

When a PR merges into `development`:
1. GitHub Actions (`docker-publish.yml`) automatically triggers.
2. A new container image is built and pushed to GitHub Container Registry (GHCR):
   - `ghcr.io/waluh-corporation/timesheet-backend:development`
   - `ghcr.io/waluh-corporation/timesheet-backend:dev-<short-sha>`
3. The staging environment deploys this image, allowing the team (2–4 engineers) to test and verify integration with other active changes before deploying to production.

---

### Phase 5: Production Release & Tagging (`v*.*.*`)

Once features integrated into `development` are thoroughly validated and ready for production deployment:

1. **Open a Release PR from `development` into `main`**:
   - **Base**: `main`
   - **Compare**: `development`
   - **PR Title**: `chore(release): release v1.11.0`
   - Update `CHANGELOG.md` following the [Keep a Changelog](https://keepachangelog.com/) standard.

2. **Merge into `main`**:
   - Wait for CI checks to pass on the release PR and merge into `main`.

3. **Create & Push Semantic Version Git Tag**:
   ```bash
   git checkout main
   git pull origin main

   # Create an annotated git tag
   git tag -a v1.11.0 -m "Release version 1.11.0"

   # Push tag to GitHub
   git push origin v1.11.0
   ```

4. **Automated Post-Tag Actions**:
   - **Production Docker Image**: Builds and tags `ghcr.io/...:v1.11.0` and `ghcr.io/...:latest`.
   - **GitHub Release**: Automatically creates an official GitHub Release with release notes derived from changes.
   - **Auto-Sync**: Workflow `sync-main-to-development.yml` ensures that `development` stays up-to-date with any release commits on `main`.

---

## 3. Semantic Versioning (SemVer) Reference

Version format: `vMAJOR.MINOR.PATCH` (e.g., `v1.11.0`)

| Bump Type | When to Use | Examples | Version Transition |
| :--- | :--- | :--- | :--- |
| **PATCH** (`x.y.Z`) | Backward-compatible bug fixes and minor adjustments. | Fixed token expiration drift, adjusted date validation error handling. | `v1.10.0` → `v1.10.1` |
| **MINOR** (`x.Y.0`) | Backward-compatible new features and additive functionality. | Added new profile request fields, added new authenticator support, new query filters. | `v1.10.0` → `v1.11.0` |
| **MAJOR** (`X.0.0`) | **Breaking changes** that require API consumers or clients to update their code. | Removed deprecated endpoints, altered mandatory JSON request/response schema contracts without fallback. | `v1.10.0` → `v2.0.0` |

---

## 4. Emergency Production Hotfixes

If a critical security vulnerability or blocking bug occurs in production:

```
main (v1.10.0) ──●──────────────● (v1.10.1)
                  ╲            ╱
                   ●── hotfix ─
```

1. Branch directly off `main`:
   ```bash
   git checkout main && git pull origin main
   git checkout -b fix/critical-security-patch
   ```
2. Implement the fix, run local tests, and push.
3. Open an urgent PR directly targeting `main`.
4. After peer review and CI pass, merge to `main` and push a new patch tag (e.g., `v1.10.1`).
5. Workflow `sync-main-to-development.yml` will automatically detect differences and open a sync PR into `development`, ensuring staging never drifts behind production fixes.

---

## 5. Daily Git Command Cheat Sheet

```bash
# === STARTING WORK / NEW FEATURE ===
git checkout main
git pull origin main
git checkout -b feat/my-feature

# === DURING DEVELOPMENT ===
go test ./...                         # Verify tests locally
git status                            # Check changed files
git add <file>                        # Stage atomic changes
git commit -m "feat(scope): message"  # Commit with Conventional Commits

# === BEFORE CREATING A PR (SYNC WITH MAIN) ===
git fetch origin
git rebase origin/main                # Keep branch linear and avoid conflicts early
git push -u origin feat/my-feature

# === AFTER MERGING PR TO DEVELOPMENT ===
# Watch GitHub Actions build the Docker image with tag 'development'

# === RELEASING TO PRODUCTION ===
git checkout main
git pull origin main
git tag -a v1.x.y -m "Release v1.x.y"
git push origin v1.x.y
```
