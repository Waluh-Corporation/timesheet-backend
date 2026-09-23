# Changelog

All notable user-facing feature updates, improvements, and fixes to the Timesheet platform are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Improved
- **Official Timesheet Formats Across All Companies**: Exported monthly timesheets now faithfully match the official spreadsheet layouts, colors, and branding for all partner companies (MII, SDD, Adidata, and NTT). Each generated file seamlessly incorporates authentic company headers, logos, and signature blocks without manual adjustments.
- **Automatic Generation Date in Signatures**: The signature section across timesheets now automatically fills the exact generation date (e.g., `DATE : 22-Sep-2026`), ensuring date fields are consistently complete without requiring manual typing.
- **Standardized Attendance Markers & Formulas**: Standardized attendance status markers for SDD timesheets to use standard status codes (`H`, `C`, `I`, `S`, `L`) instead of checkmarks (`v`), and aligned NTT summary row formulas to use `COUNTIF` matching other partner companies.

### Fixed
- **Total Attendance Calculation in Exported Timesheets**: Fixed an issue where the "Total Kehadiran" summary row remained 0 in generated Excel files even when attendance entries existed. The export engine now populates pre-calculated attendance totals alongside dynamic formulas and activates calculation properties upon opening.
- **Adidata Timesheet Template Alignment**: Fixed a layout mismatch in generated Adidata timesheets where header metadata overwritten merged cells, data rows were shifted by one row, and working hours, attendance matrix, formulas, and signatures targeted incorrect cell coordinates. Exported Adidata timesheets now faithfully match the official master template layout, cell merges, and formulas.
- **Accurate Working Hours and Total Hours Display**: Fixed an issue where regular working days without logged activities displayed confusing decimal fractions instead of default standard office hours (`08:00` and `17:00`). Daily and total working hours now consistently format as standard times (such as `09:00` or `11:00`), and weekends or holidays remain cleanly blank.
- **Accurate App Impacted & Project Selection**: Fixed an issue where daily activities could display the wrong impacted application or project variant (such as showing Overseas or Bisnis instead of Cash) in the historical activity table and edit activity modal. Project selections and impacted applications are now consistently preserved and displayed accurately across all screens.

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

---

## [1.9.0] - 2026-09-20

### Added
- **Official Yubico Hardware Authenticator Support**: Integrated official hardware AAGUID specifications covering 70 production models across the YubiKey 5 Series (NFC, USB-A, USB-C, Nano, Lightning / 5Ci), YubiKey 5 FIPS Series, YubiKey Bio Series (FIDO and Multi-protocol Editions), and Security Key by Yubico. Models are clearly distinguished by edition and profile (Consumer vs Enterprise Profile).

### Improved
- **Streamlined Passkey Brand Icons**: Simplified passkey icon delivery to a single, unified `icon` field across all credential and authenticator endpoints, reducing API response payload size and eliminating redundant theme-specific duplicates.
- **Production Hardware Whitelisting**: Strictly filtered out non-production hardware, pre-release identifiers, and preview test keys to ensure only official production hardware keys appear in the authenticator directory.
- **Updated API Specifications**: Updated Swagger/OpenAPI documentation and client contracts to reflect the unified `icon` property.

---

## [1.8.0] - 2026-09-20

### Added
- **Automatic Passkey Brand & Icon Recognition**: When registering a passkey, the system now automatically recognizes your device or password manager (such as Apple iCloud Keychain, Google Password Manager, Windows Hello, 1Password, or Bitwarden) and displays its official logo in both light and dark themes.
- **Custom Passkey Nicknames**: You can now rename your registered passkeys at any time (e.g., "Work MacBook", "Personal iPhone") to easily distinguish between multiple devices.
- **Always Up-to-Date Authenticator Catalog**: Administrators can now synchronize the system's authenticator catalog with the official passkey community registry with a single click, keeping device names and brand logos up to date.
- **Authenticator Directory Search**: Administrators can easily browse and search through supported passkey authenticators by name to verify device support.
- **Real-Time Icon Synchronization**: When brand logos are updated in the catalog, your passkeys automatically display the latest icons immediately without requiring any re-registration.
- **Pre-Check for Password Reset Links**: The password reset screen now immediately verifies whether your reset link is still valid before you type a new password, providing helpful warnings if the link has expired or was already used.

### Security & Privacy
- **Strict Passkey Privacy Protection**: Passkeys remain strictly personal and private. Administrators cannot view, modify, or delete your passkeys, ensuring full credential ownership and protection against unauthorized account access.
- **Safe External Catalog Downloads**: Synchronization with the community registry is secured with strict timeouts and size limits to prevent system slowdowns or disruptions.

---

## [1.7.1] - 2026-09-19

### Fixed
- **Instant Password Reset & User Creation Response**: Fixed a delay where requesting a password reset or creating a new user took a long time if email delivery was slow. Requests now complete instantly while confirmation emails are sent smoothly in the background.
- **Smarter Email Service Check**: Prevented system delays by skipping email sending attempts immediately when email delivery is not configured.

---

## [1.7.0] - 2026-09-19

### Added
- **Seamless Secure Sessions**: Extended login sessions with automatic background renewal so you stay securely logged in without interruptions, while ensuring that old sessions cannot be reused.
- **True Account Logout**: Logging out now terminates your session across all devices immediately.
- **Instant Account Deactivation**: Suspended or deactivated accounts lose access immediately across all active sessions.
- **Automated Public Holiday Sync**: National holidays are now updated weekly in the background, ensuring timesheets always reflect the latest holiday schedule without delays.
- **System Health Monitoring**: Added real-time health checks to ensure database and service performance remain optimal.

### Improved
- **Faster Timesheet Generation**: Generating monthly Excel timesheets is now significantly faster and no longer affected by external network slowness.
- **Clearer, User-Friendly Error Messages**: System messages and validation warnings are now written in clear, polite, and readable sentences without technical error codes.
- **Cleaner Email Notifications**: Streamlined email notifications for timesheets, reminders, and account alerts with a cleaner, clutter-free design.
- **Faster Search in Master Data**: Searching for projects, departments, approvers, and companies is now much faster and responsive.
- **Enhanced System Reliability**: Added safeguards to prevent email or push notification delays from slowing down the rest of the application.

### Security
- **Enhanced Token Protection**: Hardened login verification to prevent token tampering and replay attacks.
- **Comprehensive Audit Logging**: Improved activity tracking for logins, logouts, passkey changes, and password updates to ensure platform accountability and security.

---

## [1.6.0] - 2026-09-17

### Added
- **Work Hours Validation**: Added friendly guidance ensuring check-out time is later than check-in time when entering daily activities and overtime.
- **Empty Timesheet Warning**: Helpful alert when trying to generate a monthly timesheet without any logged activities, reminding you to fill in your work days first.
- **Restoration of Inactive Records**: Administrators can now inspect and reactivate previously archived companies, departments, sites, or divisions.
- **Flexible Multi-Domain Access**: Enabled flexible access configuration so teams across different web domains can connect smoothly.

### Fixed
- **Accurate Administrative Listings**: Ensured archived and active records are consistently visible and filterable in administration management screens.
- **Request Limiter Improvements**: Prevented unintended access blocks during development and high-traffic periods.

### Security
- **Hardened Data Queries**: Strengthened backend data handling to safeguard user notification subscriptions against tampering.

---

## [1.5.0] - 2026-09-15

### Added
- **Customizable Reminder & Cleanup Schedules**: Timesheet reminders and routine data maintenance schedules can now be tailored to match company working hours.
- **Resilient Background Automation**: Improved automated task handling with automatic recovery if a schedule setting is misconfigured.

---

## [1.4.0] - 2026-09-15

### Added
- **Work Locations & Divisions Management**: Added comprehensive support for organizing users and departments by work site (e.g., Citicon, RDTX) and corporate divisions.
- **Division-Based Department Filtering**: Filter departments conveniently by their parent division when assigning staff or viewing reports.
- **Company & Approver Directory Management**: Added dedicated views for administrators to manage company profiles and timesheet signers.
- **Adjustable Traffic Protection**: Protected system responsiveness by allowing administrators to tune access thresholds.

### Improved
- **Readable User Information**: Account and employee profiles now clearly display full organization names (site, division, department, company) across all screens and exported documents.

### Fixed
- **Deactivated Users Management**: Deactivated accounts now remain visible in administrative user management for auditing and reactivation.

---

## [1.3.0] - 2026-09-14

### Added
- **Instant Profile Loading on Login**: Your full profile and company details are now loaded immediately upon logging in, making the dashboard feel instant and responsive.

### Improved
- **Reliable Email Delivery**: Added automatic retries with smart delays to ensure important notification emails arrive reliably even during temporary network interruptions.
- **Smooth System Shutdowns**: Background tasks now shut down cleanly during maintenance windows without interrupting active user requests.

---

## [1.2.0] - 2026-09-13

### Added
- **Direct User Invitation by Admin**: Administrators can quickly create user accounts with strong generated passwords and automatic welcome emails.
- **Self-Service Password Change**: Users can securely change their password directly from account settings with email confirmation alerts.
- **Modern, Mobile-Friendly Email Notifications**: Redesigned all notification emails (account setup, password resets, reminders, and timesheet delivery) with clean mobile-responsive layouts and easy-to-tap buttons.
- **Automatic Approver Signatures in Timesheets**: Timesheet documents now automatically fill in the correct manager and approver names based on company rules.

### Improved
- **Cross-Company Departments**: Departments can now span multiple company entities, supporting flexible organizational structures.
- **Clearer Onboarding Emails**: Welcome emails now display initial credentials clearly with straightforward next steps.

### Security
- **Next-Generation Password Protection**: Upgraded password encryption to industry-leading standards to safeguard user accounts.
- **Account Duplication Prevention**: Enforced unique usernames and email addresses to eliminate accidental account collisions.

---

## [1.1.0] - 2026-09-12

### Added
- **Timesheet Author Metadata**: Exported Excel timesheets now automatically record author and company information in document properties.
- **Detailed Activity View**: View complete details of any logged daily activity, including project assignment and working hours.
- **Passkey Biometric & Security Key Support**: Full support for logging in password-free using fingerprint, face recognition, or security keys.

### Security
- **Strict Personal Activity Privacy**: Users can only view and edit their own activities, ensuring private work records cannot be accessed by other users.

---

## [1.0.0] - 2026-09-11

### Added
- **Automated Multi-Company Timesheets**:
  - Automatically generate professional Excel timesheets tailored to corporate formats (MII, NTT, SDD, and Adidata) with exact formulas, logos, and correct day counts.
  - Export timesheets directly to print-ready PDF documents.
- **Password-Free & Secure Logins**:
  - Sign in quickly using biometric passkeys (fingerprint, face unlock) or strong passwords.
  - Role-based permissions ensuring users and administrators have appropriate access levels.
- **Initial Setup Wizard**:
  - Simple first-time setup process to create the initial administrator account and configure platform settings.
- **Indonesian National Holidays**:
  - Automatically loads and synchronizes official Indonesian public holidays and collective leave (cuti bersama) to mark non-working days accurately.
- **Daily Activity Tracking**:
  - Log daily tasks with assigned client projects, start times, and end times.
  - Browse past activity logs with date range and page navigation.
- **Overtime Tracking (Surat Perintah Lembur)**:
  - Submit and track overtime hours with designated Team Leader and Department Head approvers.
- **Company & Organization Directories**:
  - Centralized management for companies, departments, projects, and work statuses.
- **Smart Reminders**:
  - Daily browser push notifications at 17:00 WIB to remind staff to fill out missing timesheet entries.
  - Email delivery for generated timesheet reports and account setup links.

[Unreleased]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.11.0...HEAD
[1.11.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.10.0...v1.11.0
[1.10.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.9.1...v1.10.0
[1.9.1]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.9.0...v1.9.1
[1.9.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.8.0...v1.9.0
[1.8.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.7.1...v1.8.0
[1.7.1]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.7.0...v1.7.1
[1.7.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.6.0...v1.7.0
[1.6.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.5.0...v1.6.0
[1.5.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.4.0...v1.5.0
[1.4.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/Waluh-Corporation/timesheet-backend/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/Waluh-Corporation/timesheet-backend/releases/tag/v1.0.0

