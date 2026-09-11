# Timesheet Backend

High-performance RESTful backend API and timesheet generation service built with Go (Gin Web Framework) and PostgreSQL.

Automates monthly timesheet generation (Excel/PDF) for multi-company formats (MII, NTT, SDD, Adidata, etc.), provides RBAC-protected endpoints, WebAuthn (Passkeys) authentication, Web Push notification reminders, and automatic Indonesian public holiday retrieval.

---

## 🚀 Key Features

### 1. Multi-Company Timesheet Generation
- **Programmatic Excel Builders**: Generates company-specific Excel timesheets (`services/builder_*.go`) with strict cell styling, company logos, day-fraction duration calculations, and automatic day-count trimming.
- **Dynamic Formula Rewriting & Signature Realignment**: Dynamically aligns formulas and signature blocks based on month lengths and working days.
- **PDF Conversion**: Converts generated Excel timesheets to landscape PDF using headless LibreOffice.

### 2. Authentication & Security
- **Dual Authentication**: Passwords hashed with bcrypt and FIDO2 / WebAuthn passkeys (`go-webauthn`).
- **Role-Based Access Control (RBAC)**: JWT authentication with `admin` and `user` role guards.
- **User Management**: Admin-managed user provisioning, profile change request workflows, and transactional email setup links.

### 3. PostgreSQL & 3NF Normalization
- **Relational Schema**: Normalized to Third Normal Form (3NF) across `companies`, `projects`, `departments`, `activity_statuses`, `holidays`, and `daily_activities`.
- **Embedded Migrations**: Versioned SQL migrations embedded directly via Go `embed` (`database/migrations/`).

### 4. Background Services & Notifications
- **Web Push (VAPID)**: Daily cron reminder at 17:00 WIB (`Asia/Jakarta`) for unsubmitted timesheets.
- **Transactional Mail**: SMTP notifications via gomail (Mailpit supported for local dev).
- **Public Holidays**: Automated synchronization with Indonesian national holiday API.

---

## 🛠️ Technology Stack

- **Language & Runtime**: Go 1.25+
- **HTTP Framework**: Gin Gonic (`github.com/gin-gonic/gin`)
- **Database & ORM**: PostgreSQL 16, GORM (`gorm.io/gorm`) with `pgx`/`postgres` driver
- **Spreadsheets**: Excelize v2 (`github.com/xuri/excelize/v2`)
- **Authentication**: `github.com/golang-jwt/jwt/v5`, `github.com/go-webauthn/webauthn`
- **Scheduler**: `github.com/robfig/cron/v3`
- **Documentation**: Swag (OpenAPI 2.0 / Swagger UI)
- **Containerization**: Alpine Linux, Docker Compose

---

## 📂 Repository Structure

```
├── assets/                 # Embedded corporate logos & templates
├── auth/                   # JWT & WebAuthn authentication service
├── cmd/                    # Administrative CLI commands
├── config/                 # Environment and runtime configuration
├── database/               # Database connection, GORM setup, and SQL migrations
│   └── migrations/         # Up/Down versioned SQL migrations
├── docs/                   # Swagger specifications and DBA administration guide
├── handlers/               # Gin HTTP route handlers and middleware
├── mailer/                 # Transactional SMTP email delivery
├── models/                 # Database entities and DTO definitions
├── push/                   # WebPush VAPID notification service
├── scheduler/              # Cron jobs (17:00 WIB reminders)
├── services/               # Excel builders, holidays, PDF conversion logic
├── templates/              # Master spreadsheet template
├── Dockerfile              # Production Go multi-stage Docker build
├── Dockerfile.dev          # Development Docker build with Air hot-reload
├── Dockerfile.prod         # Production standalone Dockerfile
├── docker-compose.yml      # Local dev stack (Postgres, Mailpit, pgAdmin, Backend)
├── docker-compose.prod.yml # Production compose configuration
├── go.mod / go.sum         # Go module dependencies
└── main.go                 # Application entry point
```

---

## 💻 Local Setup & Development

### Prerequisites
- [Go 1.25+](https://go.dev/)
- [Docker](https://www.docker.com/) and [Docker Compose](https://docs.docker.com/compose/)

### 1. Running with Docker Compose (Recommended)
Launch the entire local stack (PostgreSQL, pgAdmin, Mailpit, and Go Backend with Air hot-reloading):

```bash
docker compose up --build
```

- **Backend API**: `http://localhost:8080`
- **Swagger Documentation**: `http://localhost:8080/swagger/index.html`
- **Mailpit Web UI**: `http://localhost:8025`
- **pgAdmin**: `http://localhost:5050` (Email: `admin@timesheet.local`, Password: `admin`)

### 2. Running Locally with Go
Ensure PostgreSQL is running, then copy the environment file:

```bash
cp .env.example .env
# Edit .env with your PostgreSQL credentials
```

Run database migrations:
```bash
go run main.go -migrate
```

Start the development server:
```bash
go run main.go
```

Or with Air hot-reloading:
```bash
air -c .air.toml
```

---

## 🧪 Testing

Run the test suite across all packages:

```bash
go test ./...
```

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
