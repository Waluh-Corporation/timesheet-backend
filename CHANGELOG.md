# Changelog

All notable user-facing feature updates, improvements, and fixes to the Timesheet platform are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- **Berita Acara Activity Explanation & Single-Witness Validation**: Enforced strict validation for attendance incident reports ("Berita Acara"): each Berita Acara activity entry strictly requires an activity explanation/description (`activity`) and exactly one assigned colleague as witness (`witness_id`). Furthermore, timesheet generation enforces that all Berita Acara entries in the month share the same witness, ensuring single-witness consistency in generated incident documents.
- **Master Template Berita Acara & Automatic Bundled Generation**: Added support for generating official attendance incident reports ("Berita Acara") from a master Word (.docx) template. When employees record activities marked as Berita Acara (`is_berita_acara` or status `BA`), the monthly generation automatically renders both the timesheet spreadsheet (.xlsx) and Berita Acara document (.docx) bundled into a single ZIP download, eliminating separate generation steps.
- **Witness Selection for Berita Acara**: Employees recording Berita Acara entries can select an active colleague with role `user` as a witness (`witness_id`). Added `GET /api/v1/berita-acara/witnesses` endpoint to browse and search eligible witnesses excluding the caller.
- **Previous Day Activity Description Autofill / Retrieval**: Added `GET /api/v1/activities/previous` endpoint allowing authenticated users to fetch their most recently recorded active daily activity details (including activity description, project, working hours, and status) prior to a given reference date (defaulting to today in WIB). This enables one-click copying and autofilling of yesterday's / previous workday's tasks into today's timesheet entry form.
- **Previous Month Timesheet Access & Entry**: Users can now access, record daily activities, log overtime, and generate timesheet spreadsheets for both the current month and the previous month (maximum 1 month back). Submissions older than the previous month or for future months are strictly validated and rejected for standard users, while administrators maintain unrestricted historical access.
- **Audit Logging & Activity Trail**: Added comprehensive immutable audit logs for administrative actions (account creation, profile modifications, deactivations, personal data anonymization, and profile change reviews) recording actors, target entities, IP addresses, user agents, and metadata.
- **UU PDP Right to be Forgotten (Personal Data Anonymization)**: Added secure data anonymization endpoint (`POST /api/v1/admin/users/:id/anonymize`) allowing administrators to scrub personally identifiable information (PII) including name, email, credentials, and passkeys while retaining historical activity records for regulatory compliance.
- **Redis TLS, ACL & Connection Pool Hardening**: Enhanced Redis integration with TLS encrypted in-flight communication, ACL username authentication, and connection pool controls (pool size, idle connections, timeouts) across the caching layer and background task queue.
- **Server-Side Token Revocation & Active Session Invalidation**: Added Redis-backed JWT blacklisting (`jti`) upon user logout and active account caching to immediately terminate revoked or deactivated sessions.

### Changed
- **Redis-Backed WebAuthn Passkey Challenge Sessions**: Lowered passkey challenge session lifespan from 5 minutes to 3 minutes and integrated Redis storage with atomic single-use consumption (`GETDEL`) and graceful in-memory fallback, enabling seamless multi-instance deployment and anti-replay protection.
- **Access Token Expiry Alignment**: Aligned JWT access token lifespan to 15 minutes (`ACCESS_TOKEN_EXPIRY_MINUTES`) while utilizing refresh tokens for long-lived sessions, reducing exposure windows in accordance with OWASP API security guidelines.
- **API Response Sanitization**: Sanitized authentication and passkey responses to return dedicated DTOs (`UserResponse`) instead of raw database entities, ensuring internal database fields are never serialized over the wire.
- **Atomic Quota Counters**: Upgraded timesheet download quota counters to execute atomically via Redis Lua scripts, eliminating race conditions during concurrent download requests.
- **Optimized Daily Reminder Scheduler**: Replaced repetitive user database queries in the reminder scheduler with an efficient single-pass query, improving scheduler throughput and database responsiveness.

## [1.13.0] - 2026-09-27

### Added
- **IP2Location Geolocation Integration**: Integrated `github.com/ip2location/ip2location-go/v9` to resolve requesting client IP addresses into approximate geographic locations (city, country) displayed in password reset and password changed security notifications. Configured via `IP2LOCATION_DB_PATH` in `.env` with thread-safe lookup and graceful fallback when the database file is not present.
- **Modernized Transactional Email Design**: Redesigned transactional emails with responsive 600px containers, brand monogram header, dynamic color-coded category badges (`Akun Baru`, `Reset Sandi`, `Timesheet Siap`, `Pengingat`, `Keamanan Akun`), dark mode support (`prefers-color-scheme: dark` and Outlook `[data-ogsc]`), and preheader whitespace padding.
- **Bulletproof Email Action Buttons**: Added Microsoft Outlook VML (`v:roundrect`) button fallbacks across notification templates ensuring consistent rendering across desktop Outlook, webmail, and mobile mail clients.

### Changed
- **Security Notification Detail Box**:
  - Replaced the "Berlaku Hingga" row in password reset emails with the requester's IP address and approximate geolocation in dedicated rows.
  - Replaced the "Status Keamanan" row in password changed notification emails with the actor's IP address and approximate geolocation in dedicated rows.
  - Removed "Status Akun" from account setup emails.

## [1.12.0] - 2026-09-26

### Added
- **Asynchronous Timesheet Generation Engine**: High-throughput background generation pipeline powered by Redis (`hibiken/asynq`) with worker concurrency limiting (`EXCEL_MAX_CONCURRENT_JOBS`). Generation requests now return `202 Accepted` with a tracking Job ID, preventing HTTP timeouts and CPU starvation during peak generation windows.
- **Magic Download Tokens & Single-User Quota Enforcement**: Single-user secure download links using Magic Download Tokens with automated quota enforcement (maximum 3 downloads per generation result) and 7-day expiration, eliminating link leakage and unauthorized sharing.
- **Autonomous Cache Evaluation & Re-issue**: Intelligent idempotency check on `POST /api/v1/timesheet/generate` that automatically inspects activity updates (`MAX(updated_at)`), profile modifications, and physical S3 file availability. Re-issues fresh download links in under 5ms without redundant Excel regeneration or S3 file duplication.
- **Fast-Path Redis Counters, Debounce Lock & URL Caching**: Redis-backed atomic quota counters, 10-second distributed debounce locking, and temporary S3 Presigned URL handling to manage network reconnects with 0 MB server file bandwidth.
- **Dedicated Secure Download Endpoint**: Added public `GET /api/v1/timesheet/downloads/:token` endpoint that validates token quota, increments access counts, and issues `302 Found` redirects directly to temporary S3 Presigned URLs.
- **Configurable Download Token Quota**: Added `TIMESHEET_DOWNLOAD_MAX_QUOTA` configuration to `.env` (default 3) allowing administrators to tune maximum downloads permitted per generation link.
- **HTTP HEAD Support on Magic Download Endpoint**: Added `HEAD` method support on `/api/v1/timesheet/downloads/:token` that returns `200 OK` for email security scanners and link preview bots without consuming user download quota.
- **S3-Compatible Object Storage & 7-Day Retention**: Automated storage integration using the official AWS SDK Go v2 supporting AWS S3, MinIO, Cloudflare R2, and Wasabi with automated daily cleanup of expired files and database records.
- **SES-Ready Rate-Limited Transactional Mailer**: Throttled outbound email delivery using token-bucket rate limiting, safeguarding SMTP servers from connection spikes and rate limit penalties.
- **Real-Time Job Notifications & Status APIs**: Web Push notifications and email alerts with direct download buttons when timesheets are ready, alongside `GET /api/v1/timesheet/jobs/:id` and `GET /api/v1/timesheet/jobs` endpoints.

### Changed
- **Asynchronous Generation Contract**: `POST /api/v1/timesheet/generate` now delegates to the background queue returning `202 Accepted` with job metadata (`TimesheetJobResponse`), while retaining backward-compatible synchronous generation if the queue is unconfigured.
- **Clean Architecture & Handler Decoupling**: Refactored HTTP handlers to adhere to Clean Architecture principles by eliminating direct database coupling from `handlers.Server` and all route handlers. Introduced pure domain entities, repository interfaces, and encapsulated business logic within dedicated domain services.
- **Official Timesheet Formats Across All Companies**: Exported monthly timesheets now faithfully match official spreadsheet layouts, colors, and branding for all partner companies (MII, SDD, Adidata, and NTT).
- **Automatic Generation Date in Signatures**: The signature section across timesheets now automatically fills the exact generation date (e.g., `DATE : 26-Sep-2026`).
- **Standardized Attendance Markers & Formulas**: Standardized attendance status markers for SDD timesheets (`H`, `C`, `I`, `S`, `L`) and aligned NTT summary row formulas to use `COUNTIF`.

### Fixed
- **Timesheet Download Counter & Browser Attachment**: Fixed download counter increment issues and added RFC 6266 `Content-Disposition: attachment` headers for immediate browser file downloads.
- **Total Attendance Calculation in Exported Timesheets**: Fixed an issue where the "Total Kehadiran" summary row remained 0 in generated Excel files.
- **Adidata Timesheet Template Alignment**: Aligned cell coordinates, formulas, and headers to match the official Adidata master template.
- **Accurate Working Hours and Total Hours Display**: Formatted daily and total working hours as standard time values (`08:00`, `17:00`) instead of decimal numbers.
- **Accurate App Impacted & Project Selection**: Fixed synchronization issues where project selections and impacted applications were reset upon updates.

---

## [1.11.0] - 2026-09-21

### Added
- **Notes on Profile Change Requests**: Users can now provide a helpful note or explanation (such as moving to a new office site or department transfer) when requesting profile updates. This gives administrators clear context when reviewing submissions.
- **Email Address Updates**:
  - **For Administrators**: Administrators can now directly update any user's email address from the user management panel to keep account contact details up to date.
  - **For Users**: Users can now request an email address change through the profile update request form, which takes effect once reviewed and approved by an administrator.

---

## [1.10.0] - 2026-09-20

### Added
- **Password Reset Rate Limiting & Cooldown**: Implemented a defense-in-depth throttling mechanism (configurable via `RESET_PASSWORD_COOLDOWN_SECONDS`, default 60 seconds) per email and IP address on the password reset endpoint to prevent email bombing, spamming, and resource exhaustion.

### Improved
- **Modular Sub-Domain Architecture**: Modularized core API handlers into dedicated domain components (Passkey/WebAuthn, Password Reset, Self-Service Profile, Admin User Operations, and Overtime Tracking), significantly improving code maintainability, isolation of responsibilities, and long-term service stability without any breaking changes to API contracts.

### Security & Privacy
- **Single Active Reset Token Policy**: Enabled atomic revocation of previous active reset tokens in a single database transaction upon issuing a new reset request. Only the most recently issued link remains valid, immediately invalidating older links to prevent unauthorized reuse.
- **Strict Anti-Enumeration Protections**: Unified API responses on the forgot-password endpoint to return consistent generic messages, ensuring external parties cannot probe or determine whether an email account is registered.

---

## [1.9.1] - 2026-09-20

### Fixed
- **Seamless Passkey Device Recognition**: Resolved an issue where newly added passkeys did not remember how they connect to your device (such as built-in fingerprint, face unlock, or security keys). The system now reliably remembers your device setup so your browser immediately opens the right sign-in prompt—like Touch ID, Windows Hello, or your security key—without extra steps or delays.

[Unreleased]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.13.0...HEAD
[1.13.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.12.0...v1.13.0
[1.12.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.11.0...v1.12.0
[1.11.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.10.0...v1.11.0
[1.10.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.9.1...v1.10.0
[1.9.1]: https://github.com/Waluh-Corporation/timesheet-backend/releases/tag/v1.9.1


