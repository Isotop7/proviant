# Agent Guidelines for Proviant

This document provides guidelines for agentic coding assistants working on the Proviant codebase.

## Build, Lint, and Test Commands

### Build and Setup
```bash
# Initialize project (install dependencies, build CSS/JS, copy assets)
task init

# Run development server
task run

# Build container image
task containerimage

# Run container
task runcontainer
```

### Quality Control
```bash
# Format Go code and tidy dependencies
task tidy

# Run all Go tests
task test

# Run a single test file
cd src && go test -race -vet=off ./path/to/package

# Run a single test function
cd src && go test -race -vet=off ./path/to/package -run TestFunctionName

# Run linter (requires podman or docker)
task check

# Run linter with specific file
cd src && golangci-lint run path/to/file.go
```

### Frontend
```bash
# Build CSS
task css

# Build JS
task js

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
- Use `go fmt` (run `task tidy` before committing)
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
- Use EAN-13 format for barcode (alphanumeric, 13 digits)
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
- **Use early returns instead of nested if-clauses** — check for error conditions first and return immediately, keeping the main logic at the base indentation level
- Example:
```go
// WRONG: nested if-clauses
if ncOk {
    if ncIsType {
        if tokenErr == nil {
            if createErr == nil {
                // main logic here
            }
        }
    }
}

// CORRECT: early returns
if !ncOk {
    return
}
if !ok {
    return
}
if tokenErr != nil {
    return
}
if createErr != nil {
    return
}
// main logic here at base level
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
- **Always run `task css` after any change to `.scss` files** — the compiled CSS is what gets served; editing SCSS without recompiling has no visible effect
- JavaScript files served from `src/assets/js/`
- Use Bootstrap for styling, Bootstrap Icons for icons

#### Service Worker Caching
- The project uses a service worker (`src/assets/js/sw.js`)
- **CSS and other non-JS assets** under `/assets/` use **cache-first** — bump `CACHE_NAME` in `sw.js` after every `task css` run (or any change to fonts, icons, or other static assets)
- **JS files** use **network-first** — they are always fetched fresh; bumping `CACHE_NAME` is NOT needed for JS changes
- After bumping `CACHE_NAME`, run `task css` (or `task js`) so the new version is deployed alongside the asset change

#### Frontend JavaScript Event Handling
- **Always use event delegation** for button/input handlers: `document.addEventListener("click", ...)` with `event.target.closest()`
- **Never capture DOM references at script load time** (e.g., `const btn = document.getElementById("btnX"); btn.onclick = ...`) — these become stale after navigation or DOM updates
- Query DOM elements inside the handler function when needed, not at module scope
- This pattern ensures handlers work reliably regardless of browser caching or SPA-like navigation
- Example:
```javascript
// WRONG: stale reference captured once
const btnSave = document.getElementById("btnSave");
btnSave.onclick = function() { ... };

// CORRECT: event delegation, always works
document.addEventListener("click", function(event) {
  if (event.target.closest("#btnSave")) {
    event.preventDefault();
    // query elements here, not at top of file
    handleSave();
  }
});
```

### Documentation
- Use godoc comments for exported functions
- Include Swagger annotations for API endpoints
- Run `task doc` to generate package documentation and API docs before commits to regenerate docs
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

## Dashboard / Portal

The home page (`/web/`) is a fully client-side rendered dashboard. The Go handler (`web/frontend.go → Root`) only provides `HasHousehold` and `Household` — no server-side tile data.

**API endpoint:** `GET /api/v1/products/stats` (JWT-protected) — returns `apiModel.ProductStatsResponse` defined in `src/models/api/stats.go`. Fields:
- `wasteCount`, `wastePercent`, `totalActive` — active/expired counts
- `totalArchived`, `uniqueArchived` — archived product counts
- `lastInsertedProduct` — name of the most recently added product
- `expiringSoon` — `[]StatsExpiringProduct` (name + date) for products expiring within the next 7 days (today inclusive), sorted ascending
- `categories` — `map[string]int` category breakdown of active products
- `expiryTrend` — `[]StatsMonthlyCount` for the next 12 calendar months

**Frontend:** `src/assets/js/homeStats.js` fetches the stats endpoint on `DOMContentLoaded` and:
1. Prepends metric tiles (via `renderTile` / `renderListTile`) into `#dashboard` before the `.chart-col` sentinel elements
2. Renders three Chart.js charts into the canvas elements already present in `home.tmpl`

**Chart.js:** bundled as `src/assets/js/chart.umd.min.js` (copied from `node_modules` via `task js`). Loaded via a `<script>` tag in `home.tmpl` before `homeStats.js`.

**Removed:** `GetUserHomeTiles` DB method and the `webparts` import from `databasecontroller.go` are gone — tile data now comes entirely from the stats API.

## Important Notes
- Always use context7 when I need code generation, setup or configuration steps, or library/API documentation. This means you should automatically use the Context7 MCP tools to resolve library id and get library docs without me having to explicitly ask
- Project uses embedded filesystems (embed) for templates and assets
- Supports both SQLite and MariaDB backends
- Uses JWT tokens for API authentication
- CORS is configurable (allow all or specific origins)
- Run `task check` before committing - golangci-lint must pass
- **Database migrations**: When creating a new database model or modifying an existing one, always add it to the `AutoMigrate` call in `src/proviant.go` (around line 188). Forgetting this will cause "no such table" errors at runtime. Example:
  ```go
  migrationError := dbHandle.AutoMigrate(
      &dbModel.Household{},
      &authentication.User{},
      &dbModel.Product{},
      &dbModel.HouseholdApplication{},
      &dbModel.YourNewModel{},  // ← always add new models here
  )
  ```
- **OpenFoodFacts caching**: Barcode lookups go through the backend proxy endpoint `GET /api/v1/products/openfoodfacts/:barcode` (JWT-protected). When `openfoodfacts.cacheEnabled: true`, responses are stored in the `open_food_facts_caches` table (`src/models/database/openfoodfacts_cache.go`) and served from there on subsequent requests. The frontend (`src/assets/js/productsCreate.js`) calls `proviant.getOpenFoodFactsData()` from `proviant.js` — **do not** reintroduce direct browser calls to `world.openfoodfacts.org`. Cache operations are in `src/controllers/database/databasecontroller.go` (`GetOpenFoodFactsCacheByBarcode`, `CreateOpenFoodFactsCache`).
