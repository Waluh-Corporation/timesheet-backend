# Changelog

All notable feature changes to the Timesheet Backend project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

---

## [1.1.0] - 2026-09-12

### Added
- Multi-Vendor Workbook Metadata: Automated configuration of Excel workbook properties setting `Creator` to "Waluh Corporation" and `LastModifiedBy` dynamically to the requesting user.
- Activity Detail Endpoint: Dedicated retrieval endpoint for individual daily activity records with complete relational project details (`GET /api/v1/activities/:id`).
- SonarCloud Quality Gate: Integration of SonarCloud project analysis for static analysis, security hotspot detection, code duplication monitoring, and quality gate badges.

### Changed
- Spreadsheet Generation Engine: Fully modularized and deduplicated spreadsheet generation across MII, NTT, SDD, and Adidata formats, resolving SonarCloud duplication density and reducing cognitive complexity below threshold.
- Activity Ownership Enforcement: Restricted daily activity retrieval strictly to the authenticated owner, returning an authorization error when accessing another user's log.
- Response Formatting: Streamlined daily activity detail payload structure to directly expose normalized project code, project name, and application impacted data.
- Database Schema Hardening: Applied strict relational foreign keys with cascade constraints, composite unique indexes, and purged redundant denormalized columns (`app_impacted`).

### Fixed
- Security Hardening: Sanitized numeric route ID parameters and applied parameterized query checks to prevent SQL injection vulnerabilities (SonarCloud S3649).
- Code Quality & Linter Compliance: Upgraded `golangci-lint` configuration to v2 format, resolved all linter warnings, and elevated automated test coverage across `services` to over 91%.

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
