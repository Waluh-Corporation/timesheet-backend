# Changelog

All notable feature changes to the Timesheet Backend project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

---

## [1.7.1] - 2026-09-19

### Fixed
- **Non-Blocking Transactional Email Dispatch**: Resolved an issue where requesting a password reset (`POST /api/v1/auth/forgot-password`) or creating a new user could hang for 30+ seconds if external SMTP servers encounter connection timeouts in containerized Docker networks. Email dispatch is now handled asynchronously in background worker goroutines, allowing HTTP endpoints to respond immediately (< 10 ms).
- **Fast-Fail on Unconfigured SMTP Host**: Added immediate configuration validation in the mailer to bypass dialing attempts when `SMTP_HOST` is unconfigured, preventing redundant retry cycles and connection timeout delays.

---

## [1.7.0] - 2026-09-19

### Added
- **Dual-Token Authentication Architecture**: Short-lived (15 min) JWT access tokens paired with long-lived (7 days) SHA-256 hashed refresh tokens (`refresh_tokens`), configurable via `ACCESS_TOKEN_EXPIRY_MINUTES` and `REFRESH_TOKEN_TTL_DAYS`.
- **Refresh Token Rotation & Family Reuse Detection**: New endpoint `POST /api/v1/auth/refresh` rotates refresh tokens on every exchange and automatically revokes all chained tokens in the family if an already-consumed token is replayed.
- **Server-Side Session Logout**: New endpoint `POST /api/v1/auth/logout` allows clients to invalidate active refresh tokens in the database upon user logout.
- **Immediate Account Revocation in AuthMiddleware**: Live database validation on every authenticated request ensuring deactivated or suspended accounts (`is_active = false`) are rejected immediately (`401 Unauthorized`) without waiting for access token expiration.
- **Dynamic Connection Pooling Configuration**: Added environment variables (`DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME_MINUTES`, `DB_CONN_MAX_IDLE_TIME_MINUTES`) for tunable PostgreSQL connection management.
- **Connection Pool Health Observability**: Real-time pool metrics (`open_connections`, `in_use`, `idle`, `wait_count`, `wait_duration_ms`) exposed on `/readyz`.
- **Background Holiday Synchronization Scheduler**: Decoupled monthly timesheet generation from external HTTP latency by introducing weekly background synchronization (`syncHolidays`) from the Kemendesa national holiday API.
- **Dynamic Swagger Documentation Host**: Configured Swagger UI and OpenAPI specifications to resolve the API host dynamically based on the incoming request URL rather than relying on a static address.

### Changed
- **Clean Architecture Repository Delegation**: Eliminated direct database queries from HTTP handlers and timesheet services, redirecting all data access through decoupled repository contracts (`UserRepository`, `TokenRepository`, `MasterRepository`).
- **Workbook Generation Performance**: Replaced inline external HTTP calls during Excel timesheet generation with local indexed database lookups.
- **Atomic Profile Change Approvals**: Wrapped administrative profile change reviews (`ReviewProfileChange`) inside atomic ACID database transactions.

### Improved
- **Client-Facing Error Message Standardization**: Standardized validation and domain error messages across all backend services (`TimesheetService`, `ActivityService`, `UserService`, and `MasterDataService`) into readable, user-friendly sentence case, eliminating internal system error prefixes (e.g. `"invalid input: "`) from JSON responses.
- **Decoupled User Error Architecture**: Introduced `domain.UserError` implementing `Unwrap()` to cleanly separate client error text from backend sentinel errors while maintaining accurate HTTP status mapping (`400 Bad Request`, `404 Not Found`, etc.).
- **Streamlined Transactional Email Layout**: Cleaned up email templates by removing redundant header titles across password changed, reset, reminder, setup, and timesheet notifications for a cleaner visual appearance.
- **Covering Index for Timesheet Range Queries**: Migration 000026 adds `idx_daily_activities_range_covering` with `INCLUDE (status, project_ref_id, start_time, end_time) WHERE is_active = true`, enabling index-only scans for monthly timesheet reporting.
- **Master Data Search Acceleration**: Added PostgreSQL trigram GIN indexes (`pg_trgm`) and functional lowercase indexes (`LOWER(code)`) across companies, departments, divisions, projects, and approvers for sub-millisecond search queries.
- **I/O Resilience & Bounded Timeouts**: Added 10-second bounded timeouts for SMTP email delivery and WebPush notifications to eliminate thread pool exhaustion risks.
- **Extensible Rate Limiting Abstraction**: Extracted `RateLimiter` interface to facilitate seamless switching between in-memory and distributed caching solutions.

### Security
- **Strict Algorithm Pinning**: Enforced cryptographic algorithm verification strictly to `HS256` in `auth.ParseToken`, eliminating algorithm confusion and `none`-algorithm vulnerabilities.
- **Structured Security Event Logging**: Integrated standard library `log/slog` structured logging for authentication successes/failures, token reuse alerts, logout events, passkey operations, and password updates.
- **Single Source of Truth Migrations**: Removed legacy raw DDL from application startup and consolidated all schema evolution in versioned migration files.

---

## [1.6.0] - 2026-09-17

### Added
- **Working Hours Order Validation**: Enforced strict validation on daily activities and overtime entries ensuring check-out time is strictly later than check-in time (`check_out > check_in`), accompanied by polite, user-friendly Indonesian error guidance.
- **Empty Timesheet Generation Guard**: Added validation on monthly timesheet generation (`POST /api/v1/timesheet/generate`) to reject requests when no activities are recorded for the period, prompting users to fill their daily entries first.
- **Configurable CORS Allowed Origins via Environment**: Added `CORS_ALLOWED_ORIGINS` environment variable supporting comma-separated origin allowlists (and wildcard `*`) so frontend developers and operators can flexibly enable cross-origin API access across development, staging, and multi-domain deployments without coupling to WebAuthn configuration.
- **Master Data Inactive Record Retrieval & Reactivation**: Enabled finding inactive companies, departments, approvers, sites, and divisions by ID to allow administrative inspection and reactivation workflows.

### Changed
- **Clean Architecture & Pure 3NF Normalization**: Refactored monolithic HTTP handlers into decoupled Service and Repository layers (`ActivityService`, `TimesheetService`, `MasterDataService`, `UserService`), normalized database schema to Pure 3NF by removing redundant project denormalization from `daily_activities`, and centralized business validations.
- **API Surface Cleanup**: Decommissioned redundant timesheet summary export, push schedule, and admin test push endpoints (`/api/v1/timesheet/summary`, `/api/v1/push/schedule`, `/api/v1/admin/push/test`) to maintain a clean, secure API contract.

### Fixed
- **Rate Limiter Debug Mode and Zero-Threshold Handling**: Resolved issue where rate limiter remained active in development/debug mode (`GIN_MODE=debug`) and when configured with `RATE_LIMIT_ENABLED=false`, `RATE_LIMIT_REQUESTS=0`, or `RATE_LIMIT_WINDOW_SECONDS=0`. The rate limiter now defaults to disabled in non-release mode, configuring requests or window to `<= 0` explicitly disables rate limiting, and middleware guards prevent unintended HTTP 429 rejections.
- **Inactive Records Visibility in Admin Directory Listings**: Ensured administrative master data listings (approvers, companies, departments, sites, divisions) return both active and inactive records by default with reliable status filtering.
- **Route Registration Duplication**: Removed duplicate route declarations and redundant variable shadowing in `main.go`.

### Security
- **Parameterized Queries in Push Handlers**: Hardened user lookup logic in push handlers using parameterized SQL queries to prevent SQL injection vulnerabilities.

### Improved
- **Modular CORS Origin Validation**: Refactored origin matching and development allowlist evaluation in `handlers/server.go` into focused helper functions to reduce cognitive complexity and streamline cross-origin security rules.

---

## [1.5.0] - 2026-09-15

### Added
- **Configurable Background Scheduler via Environment**: Added environment variables (`SCHEDULER_REMINDER_CRON` and `SCHEDULER_CLEANUP_CRON`) allowing operators to customize execution schedules for daily Web Push timesheet reminders and database token housekeeping without code changes.
- **Resilient Cron Fallback & Task Control**: Implemented automatic fallback to default cron expressions upon encountering invalid syntax, along with support for explicitly disabling background jobs (`disabled`, `off`, `false`, or `none`).

---

## [1.4.0] - 2026-09-15

### Added
- **Site & Division Master Data APIs**: New directory endpoints (`GET /api/v1/sites`, `GET /api/v1/divisions`) and full administrative CRUD endpoints (`/api/v1/admin/sites`, `/api/v1/admin/divisions`, `/api/v1/admin/departments`) to manage company work locations and organizational divisions.
- **Relational Schema Integrity**: Migration 000024 introducing `sites` and `divisions` tables with foreign keys on `users`, `departments`, and `profile_change_requests`, complete with automated relational data backfill.
- **Cascading Department Division Filter**: Support for `?division=...` and `?division_id=...` query filters on `GET /api/v1/departments`.
- **Admin Master Data List Endpoints**: Administrative list endpoints for companies (`GET /api/v1/admin/companies`) and approvers (`GET /api/v1/admin/approvers`).
- **Configurable Rate Limiter via Environment**: Added environment-driven configuration for authentication route rate limiting (`RATE_LIMIT_ENABLED`, `RATE_LIMIT_REQUESTS`, and `RATE_LIMIT_WINDOW_SECONDS`), allowing operators to adjust request limits and window thresholds dynamically without redeploying code.

### Changed
- **Human-Readable User Responses**: User and profile DTO responses now return descriptive string names (`site`, `division`, `department`, `company`) alongside relational foreign key IDs, preserving seamless frontend display and spreadsheet generator compatibility.
- **Expanded API Documentation & Postman Collection**: Full 100% test coverage across all 65 Swagger endpoints with 71 automated Postman test cases.

### Fixed
- **Inactive Users Visibility in Admin Directory**: Returned both active and inactive users by default in `GET /api/v1/admin/users` to prevent deactivated accounts from disappearing from admin management screens, with optional `is_active` and `include_inactive` filtering.

---

## [1.3.0] - 2026-09-14

### Added
- **User Profile in Login Responses**: Included user profile entity in `LoginResponse` across standard password and WebAuthn/passkey login flows (`POST /api/v1/auth/login`, `POST /api/v1/passkey/login/finish`) to eliminate redundant initial profile requests.

### Changed
- **Activity Layer Architecture**: Refactored activity domain into decoupled repository and service layers (`ActivityRepository`, `ActivityService`) for streamlined business logic and improved maintainability.
- **Mailer Reliability with Auto-Retry**: Added exponential backoff retry logic and automatic `Message-ID` & `Date` header injection to transactional email delivery.
- **Graceful Rate Limiter Lifecycle**: Added graceful cleanup handling to IP rate limiting background workers during server shutdown.

---

## [1.2.0] - 2026-09-13

### Added
- **Admin User Provisioning Endpoint**: Direct administrative user creation (`POST /api/v1/admin/users`) with CSPRNG password generation, 409 Conflict duplicate checks, and welcome email dispatch.
- **Self-Service Change Password Endpoint**: Authenticated user password update endpoint (`POST /api/v1/users/change-password`) with current password verification and automated security notification email alerts.
- **Modern Responsive Email Notification System**: Redesigned transactional emails (account setup, password reset, timesheet delivery, daily reminder, and password change confirmations) with mobile-responsive layouts, localized expiration timestamps, and secure fallback links.
- **Automated Timesheet Approver Filling**: Timesheet workbooks across all company templates now dynamically resolve and populate designated approver names from the master approver directory.

### Changed
- **Flexible Department Management**: Decoupled departments from single-company constraints, enabling departments to span multiple companies, and isolated administrative roles from company assignments.
- **Optimized API Payloads**: Streamlined response structures for daily activities, overtimes, and admin user listings to reduce payload size and enhance client performance.
- **Welcome Email Redesign**: Refreshed onboarding email template to present initial login credentials clearly and remove activation steps for administrator-created accounts.

### Security
- **Argon2id Password Hashing**: Upgraded password hashing architecture to Argon2id across the entire application with transparent legacy hash verification.
- **API Protection & Origin Validation**: Restricted CORS origins to configured allowlists, prevented host header poisoning via `X-Forwarded-Host` validation, and enforced rate limiting across all authentication and password reset routes.
- **Account Uniqueness & Schema Hardening**: Enforced database-level unique constraints on email and username to prevent account collisions.

---

## [1.1.0] - 2026-09-12

### Added
- **Multi-Vendor Workbook Metadata**: Automated configuration of Excel workbook properties setting `Creator` to "Waluh Corporation" and `LastModifiedBy` dynamically to the requesting user across all generated timesheet workbooks.
- **Activity Detail Endpoint**: Dedicated retrieval endpoint for individual daily activity records with complete relational project details (`GET /api/v1/activities/:id`).
- **Activity Ownership Protection**: Restricted daily activity retrieval and management strictly to the authenticated owner to safeguard private activity entries from unauthorized access.
- **Passkey Authentication Enhancements**: Full support for discoverable and user-scoped WebAuthn/FIDO2 passwordless login ceremonies.


---

## [1.0.0] - 2026-09-11

### Added
- Multi-Company Spreadsheet Generation:
  - Automated company-specific Excel timesheet generation for MII, NTT, SDD, and Adidata formats with corporate cell styling, formulas, and company logos.
  - Automatic month-length trimming to adjust row ranges dynamically for months with fewer than 31 days.
  - Headless LibreOffice integration for automated landscape PDF export.
- Dual Authentication & Security:
  - FIDO2 / WebAuthn passkey authentication support using biometric and hardware authenticators.
  - Argon2id password hashing with automatic transparent migration from legacy hashes.
  - Role-Based Access Control (RBAC) middleware guarding admin and user endpoints.
  - Password policy enforcement adhering to NIST SP 800-63B guidelines.
- First-Time Onboarding:
  - System setup initialization endpoints (`GET /api/v1/setup/status` and `POST /api/v1/setup/init`) to bootstrap the first administrator account and default platform settings.
- Indonesian Public Holiday Synchronization:
  - Automated integration with the Kemendesa Public Holiday API (`api.kemendesa.link/libur-nasional`) with civic, religious, and joint leave classifications.
  - Yearly caching mechanism and manual holiday synchronization endpoint (`POST /api/v1/holidays/sync`).
- Daily Activity Logging & History:
  - Daily activity entry logging with direct relational linkage to billable projects.
  - Activity history listing with date range filtering and page-based pagination (`limit`, `page`).
- Overtime Management:
  - Overtime logging for Surat Perintah Lembur (SPL) reporting with designated Team Leader and Department Head approver assignments.
- Master Data Management:
  - Administrative endpoints for managing company entities and approver directories.
  - Public and authenticated directory endpoints for companies, departments, projects, and timesheet activity statuses.
- Automated Reminders & Notifications:
  - Scheduled daily Web Push notification at 17:00 WIB reminding users who have not yet submitted daily timesheet entries.
  - Transactional email dispatch for generated timesheet workbooks, account setup links, and password reset requests.
- Standardized API Response Envelopes:
  - Uniform API response envelope structure (`code`, `status`, `data`) across all endpoints under the `/api/v1` namespace.

### Changed
- API Route Versioning: Migrated all routes to the `/api/v1` prefix and decommissioned legacy unversioned endpoints.
- Decoupled Workbook Engine: Replaced database-stored template grids with dedicated programmatic spreadsheet builders.

[Unreleased]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.7.0...HEAD
[1.7.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.6.0...v1.7.0
[1.6.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.5.0...v1.6.0
[1.5.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.4.0...v1.5.0
[1.4.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/Waluh-Corporation/timesheet-backend/releases/tag/v1.0.0

