# Agent Guidelines for Proviant

This document provides guidelines for agentic coding assistants working on the Proviant codebase.

## Build, Lint, and Test Commands

### Build and Setup
```bash
# Initialize project (install dependencies, build CSS/JS, copy assets)
make init

# Run development server
make run

# Build container image
make containerimage

# Run container
make runcontainer
```

### Quality Control
```bash
# Format Go code and tidy dependencies
make tidy

# Run all Go tests
make test

# Run a single test file
cd src && go test -race -vet=off ./path/to/package

# Run a single test function
cd src && go test -race -vet=off ./path/to/package -run TestFunctionName

# Run linter (requires podman or docker)
make check

# Run linter with specific file
cd src && golangci-lint run path/to/file.go
```

### Frontend
```bash
# Build CSS
make css

# Build JS
make js

# Run frontend tests
npm test
```

## Code Style Guidelines

### Imports
- Group imports in order: stdlib → internal packages (codeberg.org) → external packages
- Use blank identifier `_` for side-effect imports (e.g., `image/jpeg`)
- Example:
```go
import (
    "fmt"
    "time"
    "codeberg.org/isotop7/proviant/errors"
    "codeberg.org/isotop7/proviant/models/database"
    "github.com/gin-gonic/gin"
)
```

### Formatting
- Use `go fmt` (run `make tidy` before committing)
- Use tabs for indentation (Go standard)
- Maximum line length: ~120 characters (not enforced but recommended)

### Naming Conventions
- **Exported functions/types**: PascalCase (e.g., `GetProduct`, `DatabaseController`)
- **Unexported functions/types**: camelCase (e.g., `setupConfig`, `handleError`)
- **Constants**: PascalCase with descriptive names (e.g., `PreferredTimeFormat`, `GeneratedPrefix`)
- **Package names**: lowercase, single words (e.g., `database`, `router`)
- **Interfaces**: PascalCase, often ending in functional description (e.g., `Authorizator`)
- **Error variables**: PascalCase with `Err` prefix (e.g., `ErrInvalidUserID`, `ErrDatabaseInvalidEngine`)

### Types and Structs
- Use GORM struct tags for database models: `gorm:"index, not null"`
- Use JSON struct tags for API responses: `json:"fieldName"`
- Use `json:"-"` to exclude fields from JSON (e.g., sensitive fields)
- Embed `gorm.Model` for ID, CreatedAt, UpdatedAt, DeletedAt
- Example:
```go
type Product struct {
    gorm.Model
    Barcode     string         `json:"barcode"`
    ProductName string         `json:"productName"`
    HouseholdID uint           `gorm:"index, not null" json:"-"`
}
```

### Error Handling
- Define custom errors in `src/errors/errors.go` as package variables
- Use predefined errors from the `errors` package instead of generic strings
- Return errors from functions (don't use `panic` in normal flow)
- Example:
```go
// Return custom error
if productID <= 0 {
    return database.Product{}, gorm.ErrNotImplemented
}
// Use custom error
if user.HouseholdID != product.HouseholdID {
    return database.Product{}, errors.ErrMismatcherUserID
}
```

### Database Operations
- Use GORM for all database queries
- Use transactions for multi-step operations: `tx := db.Begin()` → `tx.Commit()` or `tx.Rollback()`
- Soft delete with GORM's `DeletedAt` field (use `.Unscoped()` for hard delete)
- Check for record not found: `if err.Error != gorm.ErrRecordNotFound`
- Example:
```go
tx := dbc.DBHandle.Begin()
if err := tx.Create(&household).Error; err != nil {
    tx.Rollback()
    return err
}
tx.Commit()
```

### API Handlers (Gin)
- Extract logger from context: `logger, _ := ctx.MustGet("logger").(*zerolog.Logger)`
- Extract DB handle from context: `dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)`
- Extract JWT claims: `claims := jwt.ExtractClaims(ctx)`
- Return appropriate HTTP status codes: `ctx.JSON(http.StatusBadRequest, api.APIResponse{...})`
- Use `ctx.MustGet` for required context values, handle `!ok` return

### Logging
- Use zerolog for all logging
- Get logger from Gin context (don't create new loggers)
- Use appropriate log levels: `Debug()`, `Info()`, `Warn()`, `Error()`
- Format messages with `Msg()` or `Msgf()`
- Example:
```go
logger, _ := ctx.MustGet("logger").(*zerolog.Logger)
logger.Error().Msg(err.Error())
logger.Info().Msg("Logging initialized")
```

### Testing
- No tests currently exist - add them for new functionality
- Use `go test` for unit tests
- Use `t.Run()` for subtests
- Test both success and error paths

### Frontend/Assets
- SCSS files in `src/templates/scss/`
- Compiled CSS goes to `src/assets/css/`
- Run `make css` to build SCSS to CSS
- JavaScript files served from `src/assets/js/`
- Use Bootstrap for styling, Bootstrap Icons for icons

### Documentation
- Use godoc comments for exported functions
- Include Swagger annotations for API endpoints
- Run `make doc` to generate package documentation and API docs before commits to regenerate docs
- Swagger annotations format: `@Summary`, `@Description`, `@Tags`, `@Router`

### Performance Considerations
- Use `strings.Builder` instead of string concatenation in loops
- Use `strings.SplitSeq` for splitting (Go 1.18+)
- Pre-calculate buffer sizes for `strings.Builder` if known
- Avoid `fmt.Sprintf` for simple string building in hot paths

## Repository Structure
- `src/api/` - API endpoints and handlers
- `src/assets/` - Static assets (CSS, JS, fonts, icons)
- `src/controllers/` - Business logic controllers
- `src/errors/` - Custom error definitions
- `src/models/` - Data models (database, API, authentication, configuration)
- `src/router/` - Gin router setup and middleware
- `src/templates/` - HTML templates with embedded assets
- `src/web/` - Frontend page handlers
- `docs/` - Generated documentation

## Important Notes
- Project uses embedded filesystems (embed) for templates and assets
- Supports both SQLite and MariaDB backends
- Uses JWT tokens for API authentication
- CORS is configurable (allow all or specific origins)
- Run `make check` before committing - golangci-lint must pass
