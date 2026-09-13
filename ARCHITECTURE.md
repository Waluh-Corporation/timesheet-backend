# Timesheet Automation Portal — Phase 2 Architecture

Phase 2 turns the Phase 1 single-page generator into a full internal portal with
role-based access control, passkeys, admin-driven dynamic templates, incremental
daily entry, SMTP delivery, and daily Web Push reminders.

```
                        ┌────────────────────────────────────────────┐
                        │            Next.js (Saweria UI)             │
   Browser  ───────────▶│  /login  /reset-password                   │
   + Service Worker     │  (admin) /users  /template-builder         │
   (Web Push)           │  (user)  /dashboard  (Handsontable grids)  │
                        └───────────────┬────────────────────────────┘
                                        │  JSON over HTTPS (Bearer JWT)
                                        ▼
                        ┌────────────────────────────────────────────┐
                        │                Go / Gin API                 │
                        │  auth (JWT + WebAuthn)   handlers           │
                        │  mailer (gomail/SMTP)    scheduler (cron)   │
                        │  push (webpush-go)       services (excelize)│
                        └───┬───────────────┬───────────────┬─────────┘
                            │               │               │
                     PostgreSQL         Mailpit/SMTP     Push Services
                     (GORM)             (email)          (browser vendors)
```

## 1. Database schema (GORM & 3NF Normalization)

Defined in `backend/models/entities.go` and managed via versioned SQL migrations in `backend/database/migrations/`:

| Model                  | Purpose                                                              |
| ---------------------- | ------------------------------------------------------------------- |
| `User`                 | Account + live profile, RBAC role, linked to `Company` & `Department`. |
| `Company`              | Master company/vendor table (e.g. MII, SDD, Adidata, NTT).           |
| `Department`           | Master organizational unit linked to `Company`.                     |
| `Project`              | Master billable project / app impacted linked to `Company`.          |
| `ActivityStatus`       | Master daily timesheet activity status codes (`P`, `S`, `PM`, etc.). |
| `Holiday`              | Calendar holidays & joint leave with optional `Company` scope.       |
| `WebAuthnCredential`   | One stored passkey per row (credential id, public key, sign count). |
| `DailyActivity`        | User's entry per day (unique on `user_id + date`), FKs to status & project. |
| `OvertimeEntry`        | Overtime task entries for SPL generation, FKs to user & approver users. |
| `PushSubscription`     | Browser Web Push subscription (endpoint + keys).                    |
| `ProfileChangeRequest` | Pending self-service profile edit awaiting admin approval.          |
| `PasswordResetToken`   | Normalized token table with `token_type`, `used_at`, and audit IPs. |

Versioned migrations run automatically via `RunMigrations(db)` in `backend/database/migrator.go`, followed by connection pool tuning (100 max connections, 25 idle) and master data seeding. Full normalization details, ERD diagram, and backup runbooks are documented in [`docs/DATABASE_ADMINISTRATION.md`](file:///docs/DATABASE_ADMINISTRATION.md).

## 2. Authentication & RBAC

- **Password login** (`/api/auth/login`): username **or** email + password
  verified against an **Argon2id** hash → signed JWT (`backend/auth`). Hashes are
  PHC-encoded adhering to OWASP recommendations.
- **Passkey login** (WebAuthn assertion): `/api/auth/passkey/login/{begin,finish}`
  using `go-webauthn/webauthn`. Registration (`/api/passkey/register/*`) requires
  an existing session, so passkeys are added from the dashboard.
- **Multi-domain passkeys**: `WEBAUTHN_RP_ORIGIN` accepts a comma-separated list
  of allowed origins, and `/.well-known/webauthn` serves the WebAuthn *Related
  Origin Requests* document listing those origins, so one relying party's
  passkeys work across several domains rather than a single origin.
- **No public sign-up.** The only account-creation path is admin-only
  `POST /api/admin/users`, which emails a setup link.
- **Password policy (NIST SP 800-63B)**: `auth.ValidatePassword` enforces an
  8–64 character length window and a common/context-specific blocklist (no
  composition rules), applied at admin create-user, self-service reset, and the
  bootstrap-admin seed. When `BOOTSTRAP_ADMIN_PASSWORD` is unset the seeder
  generates a strong, policy-compliant password and logs it once.
- **Admin self-protection**: an admin can never deactivate, delete, or demote
  their own account (`DELETE`/`PATCH /api/admin/users/:id` reject self-targeting),
  preventing an accidental last-admin lockout.
- **Forgot/Reset password**: `/api/auth/forgot-password` issues a hashed,
  time-limited token emailed as a link; `/api/auth/reset-password` consumes it.
  Setup/reset links are built from the **public** request origin
  (`X-Forwarded-Proto`/`X-Forwarded-Host` behind a reverse proxy), falling back
  to `FRONTEND_URL`, so emailed links open on the real domain rather than a
  localhost default.
- **Route protection**: `AuthMiddleware` (Bearer JWT) + `AdminOnly` guard the
  `/api/admin/*` group. The frontend mirrors this with `Guard` on route-group
  layouts.
- **Profile approval flow**: users submit `POST /api/profile/change` →
  `ProfileChangeRequest` (status `pending`); an admin approves/rejects at
  `/api/admin/profile-changes/:id/review`, and approval copies the values onto
  the live `User`.

## 3. Dynamic template management

1. Admin uploads an `.xlsx` (`POST /api/admin/templates`). Raw bytes are stored
   on the `Template` row so generation is self-contained.
2. The workbook is parsed to a 2-D grid (`GET /api/templates/:id/grid`) and
   rendered in **Handsontable** on `/template-builder`.
3. The admin clicks a cell/column and assigns a **field** (`date`, `time_in`,
   `activity`, `meta_name`, …) with a **scope**:
   - `cell` — a single absolute cell (header metadata).
   - `daily_column` — a column whose rows repeat one-per-day, anchored at
     `start_row`; `fillable` decides whether users may edit it.
4. Mappings are saved atomically (`POST /api/admin/templates/:id/mappings`),
   replacing the prior set. Users can never reach any template-write route.

**Built-in company templates.** Two bundled templates are seeded on first boot,
each rendered by a dedicated strict-typed generator keyed off `Template.Builtin`
(`backend/services/template.go`):

- **MII Timesheet** (`builtin=mii`, **default**): working-hours block (`B` start,
  `C` end), a computed **total-hour** column `D` (`=C-B`), the `E:J` status
  matrix, activity/remark `K`, a fixed-choice **Aplikasi Terdampak** column `N`
  (Bisnis / Cash / Overseas), and hardcoded metadata — project `BNI Direct` (`L`),
  project id `P24015` (`M`), divisi `Wholesale Digital Delivery` (`P`), and
  departement `Wholesale Channel and Service Delivery` (`Q`).
- **BNI DEV Timesheet** (`builtin=bni_dev`, non-default): the second company's
  layout.

## 4. Daily entry & restricted monthly grid

- **Daily modal** (`DailyModal`): friendly "Today's Activity" / past-date entry,
  upserted via `POST /api/activities` (unique on `user_id + date`).
- **Monthly grid**: `/dashboard` renders one row per calendar day in
  Handsontable. `cells()` marks a column read-only unless its field is a
  `fillable` `daily_column` in the default template mapping. Edits auto-save.

## 5. File generation & SMTP delivery

`POST /api/timesheet/generate`:

1. Loads the chosen (or default) template + its mappings.
2. `services.GenerateFromTemplate` injects metadata cells and writes each day's
   activity down the mapped daily columns using `excelize`.
3. **Row trimming**: days beyond the month's length are cleared, so a fixed
   31-row template block is trimmed to the real month length.
4. The `.xlsx` is streamed back for direct download **and** emailed to the
   user's registered address as an attachment (async, via `gomail`/SMTP).

## 6. Web Push daily reminder

- **VAPID**: `push.Service` uses configured keys or generates a pair at boot
  (logged for pinning). Public key served at `/api/v1/push/vapid-public-key`.
- **Subscription**: `/api/v1/push/subscribe` persists the browser subscription.
- **Scheduler**: `robfig/cron/v3` with `cron.WithLocation(Asia/Jakarta)` fires
  `0 17 * * *` — **17:00 WIB daily** — and pushes
  "Waktunya isi timesheet hari ini!" to every active user with no entry for the
  day, via `SherClockHolmes/webpush-go`.
- **Service worker** (`frontend/public/sw.js`): handles `push` and
  `notificationclick`, showing a native notification and focusing/opening the
  dashboard.

## 7. Deployment

- **Single unified image (production)**: the multi-stage root `Dockerfile`
  builds the Next.js static export (`output: export`) and the Go binary into one
  Alpine image. The Go server serves the API under `/api/v1/*` **and** the exported
  frontend from the same origin — a `NoRoute` SPA handler resolves routes to the
  exported `login.html` / `dashboard.html` / … documents (with safe path
  handling), so no separate web server or second container is needed.
  `docker-compose.prod.yml` runs just this image + PostgreSQL.
- **Dev stack** (`docker-compose.yml`): PostgreSQL, pgAdmin (DBeaver can also
  attach on 5432), Mailpit (SMTP + web UI on 8025), and hot-reloading backend +
  frontend as separate containers.
- `deploy_and_commit.sh`: validates a Conventional Commit, commits + pushes,
  SSHes to `192.168.0.2:222`, **scans upward from port 2000** for a free port on
  the host, and brings the stack up there.

## API surface & Swagger Documentation

Interactive Swagger / OpenAPI 2.0 documentation is served directly by the Go backend at:
- **Swagger UI**: `http://localhost:8080/swagger/index.html` (or redirect via `/swagger`)
- **OpenAPI JSON Spec**: `http://localhost:8080/swagger/doc.json`

| Method & path                              | Auth   | Purpose                        |
| ------------------------------------------ | ------ | ------------------------------ |
| `GET  /swagger/*any`                       | public | Interactive Swagger UI & OpenAPI spec |
| `POST /api/auth/login`                     | public | Password login                 |
| `POST /api/auth/passkey/login/{begin,finish}` | public | Passkey login               |
| `POST /api/auth/forgot-password`           | public | Request reset link             |
| `POST /api/auth/reset-password`            | public | Set new password via token     |
| `GET  /api/me`                             | user   | Current profile                |
| `POST /api/profile/change`                 | user   | Submit profile change (pending)|
| `GET  /api/profile/changes`                | user   | My profile change requests     |
| `POST /api/passkey/register/{begin,finish}`| user   | Add a passkey                  |
| `GET|DELETE /api/passkeys[/:id]`           | user   | List or delete my passkeys     |
| `POST /api/activities`                     | user   | Upsert a day                   |
| `GET  /api/activities`                     | user   | List activities (paginated)    |
| `GET  /api/activities/:id`                 | user   | View activity detail by ID     |
| `POST|GET|DELETE /api/overtimes[/:id]`     | user   | Manage overtime entries for SPL|
| `POST /api/timesheet/generate`             | user   | Generate + email `.xlsx`       |
| `GET  /api/holidays`                       | user   | Public holidays for calendar   |
| `GET  /api/projects`                       | user   | List active normalized projects|
| `GET  /api/companies`                      | user   | List companies & relations     |
| `GET  /api/departments`                    | user   | List normalized departments    |
| `GET  /api/activity-statuses`              | user   | List activity status codes     |
| `GET  /api/templates`                      | user   | List templates + mappings      |
| `GET  /api/templates/:id/grid`             | user   | Parsed grid for Handsontable   |
| `POST /api/push/subscribe`                 | user   | Save push subscription         |
| `POST /api/push/unsubscribe`               | user   | Delete push subscription       |
| `POST /api/push/test`                      | user   | Dispatch test push notification|
| `GET  /api/admin/users` / `POST`           | admin  | List / create users            |
| `PATCH|DELETE /api/admin/users/:id`        | admin  | Update / delete user           |
| `GET|DELETE /api/admin/users/:id/passkeys` | admin  | Manage user passkeys           |
| `GET  /api/admin/profile-changes`          | admin  | Pending profile changes        |
| `POST /api/admin/profile-changes/:id/review` | admin| Approve / reject               |
| `POST /api/admin/templates`                | admin  | Upload template                |
| `POST /api/admin/templates/:id/mappings`   | admin  | Save cell mappings             |
| `POST /api/admin/templates/:id/default`    | admin  | Set default template           |
| `DELETE /api/admin/templates/:id`          | admin  | Delete template                |

