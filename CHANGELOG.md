# Changelog

All notable feature changes to the Timesheet Backend project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- **Admin User Provisioning Endpoint**: Direct administrative user creation (`POST /api/v1/admin/users`) with CSPRNG password generation, 409 Conflict duplicate checks, and welcome email dispatch.
- **Self-Service Change Password Endpoint**: Authenticated user password update endpoint (`POST /api/v1/users/change-password`) with current password verification and security notification alert.
- **Argon2id Hashing Engine**: Upgraded password hashing architecture to Argon2id across the entire repository with transparent legacy hash verification.
- **Database Hardening Migration**: Migration `000022` expanding `password_hash` column to `VARCHAR(255)` and enforcing unique constraints on `email` and `username`.
- **Dynamic Application Configurations**: Added `APP_NAME`, `FRONTEND_URL`, and `ADMIN_EMAIL` environment configurations.

### Changed
- **Welcome Email Template**: Updated onboarding email layout to present initial credentials clearly and remove activation requirements for admin-provisioned accounts.
- **CI/CD Workflow**: Configured CI quality gate pipelines to run on Pull Requests targeting the `development` branch.

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

[Unreleased]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/Waluh-Corporation/timesheet-backend/releases/tag/v1.0.0

