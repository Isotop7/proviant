# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Proviant is a food expiration tracking web app built with Go (Gin, GORM) on the backend and Bootstrap 5 on the frontend. It supports SQLite and MariaDB, JWT authentication, and notifications via SMTP and Ntfy.sh.

## Commands

```bash
task init          # Install npm deps, build CSS/JS, copy assets
task run           # Start development server
task tidy          # go fmt + go mod tidy
task test          # Run all Go tests
task check         # Run golangci-lint (requires podman or docker)
task css           # Compile SCSS → CSS
task js            # Copy JS files to assets
task doc           # Generate package docs + Swagger API docs
```

**Single test:**
```bash
cd src && go test -race -vet=off ./path/to/package -run TestFunctionName
```

**Must pass before committing:** `task tidy` (formatting) and `task check` (linting).
**After API changes:** run `task doc` to regenerate Swagger docs.

## Architecture

**Request flow:** Gin router → JWT middleware → API handler or web handler → DatabaseController → GORM → SQLite/MariaDB

**Key layers:**
- `src/api/` — REST API handlers (`auth/`, `common/`, `v1/products`, `v1/user`)
- `src/web/` — SSR frontend page handlers (render HTML templates)
- `src/controllers/database/` — All GORM queries via `DatabaseController`
- `src/controllers/` — `NotificationController` (scheduled goroutine), `OpenFoodFactsAPIController`
- `src/router/` — Gin router, middleware (JWT, logger/DB injection into context), CORS
- `src/models/` — GORM models (`database/`), JWT/user (`authentication/`), config structs (`configuration/`)
- `src/errors/errors.go` — All custom error variables (define new ones here)
- `src/templates/` — Embedded HTML templates and SCSS; `src/assets/` — compiled static files

**Context injection:** Middleware puts logger, DB handle, and controllers into Gin context. Handlers extract them:
```go
logger, _ := ctx.MustGet("logger").(*zerolog.Logger)
dbHandle, _ := ctx.MustGet("dbHandle").(*gorm.DB)
claims := jwt.ExtractClaims(ctx)
```

## Code Conventions

**Import order:** stdlib → internal (`codeberg.org/isotop7/proviant/...`) → external packages

**Error handling:** Use predefined errors from `src/errors/errors.go`; never `panic` in normal flow.

**Database:** Use GORM transactions for multi-step operations; soft delete via `DeletedAt` (`.Unscoped()` for hard delete).

**Logging:** Always use zerolog from Gin context — never create new loggers.

**Models:** Embed `gorm.Model` for standard fields; use `gorm:"index,not null"` and `json:"fieldName"` tags; `json:"-"` to exclude sensitive fields.

**Frontend:** SCSS lives in `src/templates/scss/`; compiled to `src/assets/css/`. Use Bootstrap for styling and Bootstrap Icons for icons.

## Documentation

Always use the Context7 MCP tools automatically (without being asked) when you need library/API documentation, code generation help, or configuration steps for any library used in this project.
