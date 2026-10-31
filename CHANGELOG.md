# Changelog

All notable user-facing feature updates, improvements, and fixes to the Timesheet platform are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

## [1.17.0] - 2026-11-01

### Added
- **Document Verification Audit Trail & Stepper Timeline**: Enhanced the public document verification API and web view with an immutable, chronologically ordered audit log trail (`AuditTrail`). Tracks key lifecycle milestones including document submission, witness approvals/rejections, SHA-256 seal generation, revocations, and verification lookups. The standalone HTML verification page renders these events in a zero-CDN, CSP-compliant Tabler vertical stepper timeline with color-coded status badges, formatted timestamps, and actor details.
- **Client IP Address Anonymization for Public Verification**: Integrated privacy-preserving IP masking (`MaskIP`) for audit trail verification entries, masking the last two octets of IPv4 addresses (`192.168.***.***`) and trailing segments of IPv6 addresses (`2001:db8:****::1`).
- **English Month Formatting Utility**: Added `FormatMonthYearEnglish` in mailer templates and `MonthNameEnglish` in service spreadsheet builders for standardized English calendar formatting across reports and email notifications.

### Changed
- **Comprehensive Email Template English Localization**: Fully translated all 8 transactional email templates (`layout`, `setup`, `reset`, `password_changed`, `reminder`, `timesheet`, `witness_request`, `berita_acara_approved`, and `berita_acara_rejected`), plain-text fallbacks, and Gomail dispatch subjects into clear, idiomatic English. Configured email timestamps to dynamically respect the application timezone (`TZ`) with proper timezone abbreviations.
- **WebPush Notification English Normalization**: Standardized all push notification titles and notification bodies (timesheet reminders, witness signature requests, and approval/rejection updates) to English.
- **Timesheet Owner Signature Approver Blanking**: Updated the Excel signature embedding engine (`WriteSignatures`) to stamp only the timesheet owner's digital signature and submission date, leaving Team Leader and Department Head approver cells blank for physical review and sign-off.

### Removed
- **Email Category Badges**: Removed top-right header category badge elements from `layout.html`, badge struct fields and styling methods from `BaseLayoutData`, and redundant badge pill markup from individual email templates.

## [1.16.0] - 2026-10-31

### Added
- **Dynamic Weekend & Holiday Overtime Timesheet Generation**: Full support for overtime logged on weekends and public holidays. Timesheet spreadsheets across all vendor formats (MII, SDD, Adidata, and NTT) now display overtime hours, project details, attendance markings, and calculated totals on clean white rows instead of inactive grey backgrounds.
- **Weekend Inactive Row Remark Standardization**: Consistent timesheet notes for inactive days off. Weekend days without logged activities now clearly show "Weekend" in the remarks column across all timesheet templates.
- **Admin Audit Log Query & Filtering**: Comprehensive administrative audit trail. Administrators can now browse and filter system audit records by date range, action type, user, and resource to monitor administrative changes.
- **Dynamic Push Notification Schedule Endpoint**: Daily reminder schedule transparency. Users and client apps can now view active daily notification reminder times, the configured timezone, and the exact time of the next scheduled reminder.
- **Digital Signature & Document Verification QR Code System**: Digital signatures and tamper-evident QR verification. Monthly timesheets and incident reports now feature embedded employee signatures alongside secure verification QR codes to guarantee document authenticity.
- **Standalone Tabler Document Verification Webpage**: Mobile-friendly document verification web page. Anyone scanning a timesheet or report QR code with a smartphone camera can instantly verify document validity, signers, signature dates, and integrity in a clean, modern web view.
- **User Profile Signature Management**: Profile signature setup. Employees can now easily draw or upload their signature directly in their profile settings for use on official documents.
- **Explicit Timesheet Sign-Off & Period Locking**: Monthly timesheet locking and sign-off. Employees can officially sign off on their completed monthly timesheet, locking activity edits to prevent accidental changes and embedding their signature and QR verification code into generated spreadsheets.
- **Asynchronous Berita Acara Dual-Signature Workflow**: Collaborative witness sign-off for incident reports (Berita Acara). Employees can send attendance incident reports to a nominated colleague for witness verification. Witnesses receive email and push notifications to review, approve, or request revisions. Once approved, the generated Word document automatically includes both employee and witness signatures with a verification QR code.
- **Dedicated Responsive Email Templates**: Action-oriented email alerts for report approvals. New clean, responsive email notifications alert colleagues when their witness signature is requested and notify employees once their report has been approved or rejected.
- **Public Document Verification API for Frontend**: Public verification service. External portals and client applications can programmatically verify document details, signer identity, and tamper status.

### Fixed
- **Timesheet Handler Test End-of-Month Calculation**: End-of-month date calculations. Resolved an edge-case calculation error when retrieving or processing records on the 31st day of the month.
- **Inadvertent Grey Styling on Weekend/Holiday Overtime Rows**: Accurate overtime row highlights. Resolved an issue where overtime rows on weekends and holidays were accidentally shaded grey instead of remaining highlighted as active workdays.
- **Attendance Status Fallback on Overtime Entries**: Reliable attendance markings for overtime. Ensured attendance columns always record present status markers on overtime days even if no explicit status code was provided.
- **Verification Page Self-Contained Styling & CSP Whitelist**: Self-contained document verification styling. Embedded styles and icons directly into the document verification page to ensure fast, reliable rendering across all mobile and desktop browsers under strict security headers.
- **Verification Page English Label Normalization**: Standardized verification page wording. Cleaned up document headings and labels for consistent English presentation.
- **Berita Acara OpenXML `.docx` Schema Corruption on Signature Embed**: Word document recovery error fix. Resolved an issue where digitally signed Berita Acara Word (.docx) files prompted a repair or recovery alert in Microsoft Word when opened.
- **Berita Acara Verification QR Relocation to Word Footer**: Clean single-page Word report layout. Moved the verification QR code into the document footer so the entire Berita Acara document fits neatly on a single printed page.
- **Berita Acara Floating Signatures ("In Front of Text")**: Natural signature placement in Word documents. Signatures now float naturally over the signature lines without pushing text down or altering paragraph alignment.
- **Strict Designated Witness Signature Binding**: Accurate witness signature placement. Guaranteed that the approved colleague's signature always attaches to the witness column without duplication or mismatch.

### Changed
- **SonarQube Quality Hardening & Cognitive Complexity Optimization**: Internal code health and performance improvements. Refactored key services and controllers to boost efficiency, streamline internal workflows, and maintain strict quality standards.
- **Inactive Attendance Status Matrix Marker Blanking**: Cleaner attendance matrix display. Inactive non-working days now leave attendance cells clean and blank instead of printing "X", matching company timesheet design guidelines.
- **Repository Interface Segregation & ACID Transactor Abstraction**: Enhanced database transaction safety. Upgraded multi-step operations (such as profile approvals and account updates) to guarantee complete data consistency.
- **Distributed Redis State for WebAuthn Sessions & Cooldowns**: High-availability authentication sessions. Passkey (WebAuthn) logins and password reset cooldowns now synchronize reliably across multiple backend servers.
- **Verification Page Configurable Timezone Display**: Timezone-aware verification timestamps. Document verification records now display timestamps in the system's configured timezone (e.g., WIB, UTC) with clear timezone abbreviations.
- **Document Verification & Berita Acara Generator Code Quality Hardening**: Document generator reliability enhancements. Streamlined document generation routines for faster, more dependable file creation.
- **Berita Acara Witness Signature Guard on Generation**: Witness approval requirement before export. Timesheet and Berita Acara exports are now protected: if a month contains incident reports, they must be reviewed and signed by the witness before the final documents can be downloaded.
- **Berita Acara Single Monthly Witness Simplification**: Simplified witness selection. Employees designate one witness per month; that choice is automatically shared across all incident entries for that month, removing repetitive selections.
- **Berita Acara Submission Monthly Witness Auto-Resolution**: Automatic witness linking on submission. Submitting an incident report automatically associates the employee's designated monthly witness if not specified individually.
- **Timesheet Delivery Email Template Refinement**: Updated delivery email wording. Download buttons and email notifications now clearly indicate that deliveries include both Timesheet spreadsheets and Berita Acara documents.
- **Daily Activity Copying Excludes Working Hours & Berita Acara**: Smarter previous-day activity copying. Copying tasks from a previous day now imports task descriptions and project names while leaving hours and incident reports blank, encouraging accurate daily time recording.
- **AuthService Extraction & Clean Architecture Separation**: Strengthened account authentication security. Centralized password verification, token refresh rotation, and login audit trails to protect against session reuse and token tampering.
- **Tabler-Inspired Transactional Email Redesign**: Modernized email notification design. Upgraded all transactional emails (account setup, password resets, timesheet deliveries, reminders) with clean modern styling, clear key-value summary cards, and full dark mode support across Outlook, Gmail, and Apple Mail.

### Removed
- **Dead Code Cleanup (Mapper & Domain Entity Layer)**: Retired obsolete internal components. Removed legacy data mappers and inactive entities to streamline system maintenance.

## [1.15.1] - 2026-10-24

### Fixed
- **Timesheet Activity to Sub Department Text Wrapping**: Preserved text wrapping across columns Activity/Remark through Sub Department (columns K through R in MII and corresponding activity/project/department columns across all company templates), preventing template wrapping from being overwritten during spreadsheet generation.

## [1.15.0] - 2026-10-22

### Changed
- **Timesheet Spreadsheet Font Size & Readability**: Enlarged the font size of timesheet daily table rows and summary totals to 14 pt across all company templates (MII, SDD, Adidata, and NTT), set table column headers to 14 pt (Bold), and adjusted upper metadata headers to 12 pt. Increased base row height to 21 pt with proportional dynamic text wrapping so larger text renders cleanly without vertical clipping.

### Security
- **Security Vulnerability Remediation**: Upgraded Go toolchain to `1.27.3` and `excelize` library to patch known security vulnerabilities in intermediate certificate trust verification (`GO-2026-6809`), spreadsheet stream allocation memory exhaustion (`GO-2026-6689`), array formula recursion stack overflows (`GO-2026-6688`), and malformed OLE/CFB decryption handling (`GO-2026-6684`).

---

## [1.14.0] - 2026-10-18

### Added
- **Berita Acara (Official Word Document Generation)**: Implemented automated Berita Acara document generation (`.docx`) utilizing Go native archive streaming and XML templating based on the corporate `Berita_Acara-Template.docx` format. Captures employee identity, department, group, team lead approver, and dynamically generates rows with Indonesian day names, dates, working hours, and activity explanations for entries marked as Berita Acara.
- **Monthly Single-Witness Locking & Integrity Protection**: Enforced strict business rules ensuring each employee has exactly one consistent witness per month for all Berita Acara records. The initial Berita Acara entry in a calendar month requires and locks the chosen colleague (`witness_id`). Subsequent entries in the same month automatically inherit the locked witness when omitted, while attempts to specify a different witness without an explicit change request are rejected. Selecting oneself, inactive accounts, or non-user roles as a witness is strictly prohibited.
- **Dedicated Witness Discovery & Management APIs**: Added `GET /api/v1/users/witnesses` returning safe public profiles of eligible peer colleagues (`RoleUser`), and `PUT /api/v1/activities/witness` allowing users to atomically reassign their designated monthly witness across all Berita Acara records for a given month in a single operation.
- **Timesheet & Berita Acara Bundled ZIP Delivery**: Upgraded the asynchronous timesheet export pipeline to check for Berita Acara records. When present, both the `.xlsx` monthly timesheet and the generated `.docx` Berita Acara are automatically bundled into a single `.zip` archive (`Timesheet_and_Berita_Acara_<username>_<MM>_<YYYY>.zip`), uploaded to S3 storage, and delivered seamlessly via secure magic download tokens and push/email alerts.
- **Previous Day Activity Description Autofill / Retrieval**: Added `GET /api/v1/activities/previous` endpoint allowing authenticated users to fetch their most recently recorded active daily activity details (including activity description, project, working hours, and status) prior to a given reference date (defaulting to today in WIB). This enables one-click copying and autofilling of yesterday's / previous workday's tasks into today's timesheet entry form.
- **Previous Month Timesheet Access & Entry**: Users can now access, record daily activities, log overtime, and generate timesheet spreadsheets for both the current month and the previous month (maximum 1 month back). Submissions older than the previous month or for future months are strictly validated and rejected for standard users, while administrators maintain unrestricted historical access.
- **Audit Logging & Activity Trail**: Added comprehensive immutable audit logs for administrative actions (account creation, profile modifications, deactivations, personal data anonymization, and profile change reviews) recording actors, target entities, IP addresses, user agents, and metadata.
- **UU PDP Right to be Forgotten (Personal Data Anonymization)**: Added secure data anonymization endpoint (`POST /api/v1/admin/users/:id/anonymize`) allowing administrators to scrub personally identifiable information (PII) including name, email, credentials, and passkeys while retaining historical activity records for regulatory compliance.
- **Redis TLS, ACL & Connection Pool Hardening**: Enhanced Redis integration with TLS encrypted in-flight communication, ACL username authentication, and connection pool controls (pool size, idle connections, timeouts) across the caching layer and background task queue.
- **Server-Side Token Revocation & Active Session Invalidation**: Added Redis-backed JWT blacklisting (`jti`) upon user logout and active account caching to immediately terminate revoked or deactivated sessions.
- **Swagger UI Production Guard & Configuration**: Added `SWAGGER_ENABLED` environment variable controlling interactive OpenAPI documentation mounting. Defaults to enabled in development and automatically disabled in production (`GIN_MODE=release`) to mitigate API schema enumeration and attack surface exposure.
- **Resource Exhaustion & DoS Rate Limiting on Timesheet Downloads and Generation**: Extended IP rate limiting to protect public timesheet download token routes (`/timesheet/downloads/:token`) and resource-intensive generation endpoints against denial-of-service abuse.

### Changed
- **Adidata Overtime (SPL) Consolidation**: Consolidated the Adidata overtime sheet from multiple per-activity sheets (`SPL-1`, `SPL-2`, etc.) into a unified single sheet (`SPL`) matching the latest corporate template (`SPL_Adidata-Template.xlsx`). All monthly overtime entries are now presented in a single tabular list with dynamic row expansion for more than three overtime activities, formula-based total duration calculation (`=SUM(...)`), and hierarchical approver signature sections.
- **Internal Server Error Masking & CWE-209 Hardening**: Hardened HTTP 500 error envelopes in `handlers/response.go` to mask raw technical error details in release mode while recording full diagnostic context to internal structured logs, preventing database schema and stack trace leakage to clients.
- **Subprocess Execution Timeout for PDF Conversion**: Wrapped LibreOffice headless spreadsheet-to-PDF conversion in `services/pdf.go` with a deterministic 30-second context timeout, guarding against hung subprocesses and resource starvation.
- **Timesheet Builders & Auth Architecture Refactoring**: Streamlined workbook generation logic across Adidata, NTT, SDD, and MII templates to reduce cognitive complexity and parameter counts, replaced duplicate string literals with typed constants, and decoupled logout token blacklisting and session revocation handlers for enhanced code maintainability and SonarCloud compliance.
- **Configurable Timezone with UTC Default**: Changed the default application and scheduler timezone to `UTC` across the configuration loader, database connections, daily cron scheduler, background mailer, and timesheet period validation services. Added first-class configuration via environment variable `TZ` (with backward-compatible fallback to `SCHEDULER_TZ`) allowing flexible deployment in any IANA timezone.
- **Responsive Email Template & Cross-Client Compatibility Hardening**: Upgraded all transactional email templates (`layout.html`, `password_changed.html`, `reminder.html`, `reset.html`, `setup.html`, `timesheet.html`) with dual-tier fluid breakpoints (`max-width: 620px` and `max-width: 480px`), centered Microsoft Outlook MSO conditional wrappers for both cards and external footers, full-width responsive action buttons (`.btn-shell` and `.btn-block`), automatic label-value row stacking on narrow smartphone viewports (`.stack-row`), cross-client Outlook font fallbacks, URL break-word wrapping, and dark mode styling across Apple Mail, Gmail, and Outlook.com/Office 365 (`[data-ogsc]` & `[data-ogsb]`).
- **Access Token Expiry Alignment**: Aligned JWT access token lifespan to 15 minutes (`ACCESS_TOKEN_EXPIRY_MINUTES`) while utilizing refresh tokens for long-lived sessions, reducing exposure windows in accordance with OWASP API security guidelines.
- **API Response Sanitization**: Sanitized authentication and passkey responses to return dedicated DTOs (`UserResponse`) instead of raw database entities, ensuring internal database fields are never serialized over the wire.
- **Atomic Quota Counters**: Upgraded timesheet download quota counters to execute atomically via Redis Lua scripts, eliminating race conditions during concurrent download requests.
- **Optimized Daily Reminder Scheduler**: Replaced repetitive user database queries in the reminder scheduler with an efficient single-pass query, improving scheduler throughput and database responsiveness.

### Fixed
- **Berita Acara DOCX Template OpenXML Compatibility**: Resolved XML namespace corruption in the master Word template (`Berita_Acara-Template.docx`) caused by invalid relationship and paragraph namespace attributes (`ns3:id` and `ns2:paraId`). Generated Word documents now open cleanly in Microsoft Word without unreadable content or recovery prompts.
- **Anonymized Account Immutability & Modification Guards**: Prevented modifications to company, department, site, or profile details (`PATCH /api/v1/admin/users/:id`), redundant deactivations (`DELETE /api/v1/admin/users/:id`), and re-anonymization on already anonymized accounts, returning `403 Forbidden`. Enhanced anonymization to transactionally revoke passkeys, push subscriptions, and pending profile change requests, and exposed explicit `is_anonymized` status in user listings.
- **Code Quality & Architecture Robustness**: Enhanced server middleware by decomposing authentication and token revocation workflows into streamlined modules, standardized internal error definitions and query identifiers across user management services and repositories, documented runtime timezone database embedding for containerized environments, and eliminated redundant evaluation expressions.

### Removed
- **Embedded Frontend / SPA Static File Serving**: Removed embedded SPA static file serving, route fallback handlers (`spaHandler`), and static path helpers (`tryFiles`, `isPathWithinRoot`) to maintain a pure, dedicated backend REST API architecture.

[Unreleased]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.17.0...HEAD
[1.17.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.16.0...v1.17.0
[1.16.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.15.1...v1.16.0
[1.15.1]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.15.0...v1.15.1
[1.15.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.14.0...v1.15.0
[1.14.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.13.0...v1.14.0
