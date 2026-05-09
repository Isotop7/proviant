# Agent Guidelines for Proviant

This document provides guidelines for agentic coding assistants working on the Proviant codebase.

## Code Search

Use `semble search` to find code by describing what it does or naming a symbol/identifier, instead of grep:

```bash
semble search "authentication flow" ./my-project
semble search "save_pretrained" ./my-project
semble search "save model to disk" ./my-project --top-k 10
```

Use `semble find-related` to discover code similar to a known location (pass `file_path` and `line` from a prior search result):

```bash
semble find-related src/auth.py 42 ./my-project
```

`path` defaults to the current directory when omitted; git URLs are accepted.

If `semble` is not on `$PATH`, use `uvx --from "semble[mcp]" semble` in its place.

## Workflow

1. Start with `semble search` to find relevant chunks.
2. Inspect full files only when the returned chunk is not enough context.
3. Optionally use `semble find-related` with a promising result's `file_path` and `line` to discover related implementations.
4. Use grep only when you need exhaustive literal matches or quick confirmation of an exact string.

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
- Use blank identifier `_` for side-effect imports (e.g., `image/jpeg`), add a comment on why they are imported
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
- **Variables**: Use full descriptive camelCase names — never abbreviate (e.g., `storageLocationRepo` not `slRepo`, `productController` not `pc`, `householdID` not `hID`)

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
- Use appropriate log levels: `Debug()`, `Info()`, `Warn()`, `Error()`, `Fatal()`
- Format messages with `Msg()` or `Msgf()`
- Example:
```go
logger, _ := ctx.MustGet("logger").(*zerolog.Logger)
logger.Warn().Msg(err.Error())
logger.Info().Msg("Logging initialized")
```

#### Log Level Policy

| Level | When to use | Examples |
|-------|-------------|----------|
| **Trace** | SQL query tracing, extremely detailed debug | GORM adapter SQL tracing |
| **Debug** | Dev-time info, cache hit/miss, migration steps, individual event processing | "migration step running", "cache hit", "recipe match attempt" |
| **Info** | Normal operational events worth recording in production | Startup, user created, config loaded, cleanup counts, notification sent, email sent |
| **Warn** | Recoverable, expected, or user-caused issues that don't require immediate action | Invalid user input (4xx), missing optional dependencies, background task failures, cache misses/errors, "not found" in user-requested lookups, file close errors |
| **Error** | Unexpected server-side failures requiring attention | DB operation failures, external API failures, missing required context values (controllers, repos), 5xx-causing errors, internal logic errors |
| **Fatal** | Immediate startup termination | Cannot bind port, cannot set trusted proxies |

##### Decision Tree for API Handlers

```
Is this a user/client error? (4xx response)
  → Warn

Is this a resource not found that the user requested?
  → Warn

Is this a background/non-critical side operation failing?
  → Warn

Is this a server-side error or unexpected failure? (5xx response)
  → Error

Is this a missing required dependency or context value?
  → Error

Is this a startup failure that prevents the app from running?
  → Fatal
```

### Testing

#### Test Setup
- Use `testutil.SetupTestDB(t)` for database setup - creates in-memory SQLite with all models migrated
- Use `testutil.SetupGinContext(db)` for API handler tests - creates test context with logger and db
- Use `t.Run()` for all subtests

#### Test Helpers (`src/testutil/`)
- `testutil.SetupTestDB(t)` - Creates in-memory SQLite DB with all migrations
- `testutil.SetupGinContext(db)` - Creates Gin test context with logger and db
- `testutil.MockJWTClaims(ctx, userID)` - Sets JWT claims with "id" key
- `testutil.MockJWTClaimsWithKey(ctx, userID, key)` - Sets JWT claims with custom key
- `testutil.CreateTestUser(db, householdID)` - Creates test user
- `testutil.CreateTestHousehold(db, adminID)` - Creates test household
- `testutil.CreateTestProduct(db, householdID)` - Creates test product
- `testutil.CreateTestStorageLocation(db, householdID)` - Creates test storage location

#### Assertions
- Use plain Go assertions: `if got != want { t.Errorf(...) }`
- Both plain Go and testify/assert are acceptable but use plain Go for consistency

#### Test Guidelines
- Use `go test` for unit tests
- Use `t.Run()` for subtests
- Test both success and error paths
- Keep test DB setup in the helper functions, not duplicated in each test file

### Frontend/Assets
- SCSS files in `src/templates/scss/`
- Compiled CSS goes to `src/assets/css/`
- **Always run `task css` after any change to `.scss` files** — the compiled CSS is what gets served; editing SCSS without recompiling has no visible effect
- JavaScript files served from `src/assets/js/`
- **Always use Bootstrap 5 for styling and layout** — Context7 library ID: `/websites/getbootstrap`
- **Always use Bootstrap Icons for icons** — Context7 library ID: `/twbs/icons`
- **Avoid inline `style="..."` attributes in templates** — define CSS classes in `src/templates/scss/main.scss` instead. Inline styles are only acceptable for truly dynamic values (e.g., a `width` set from a template variable). When in doubt, use a class.

#### Bootstrap & Bootstrap Icons — Documentation Workflow
When implementing or modifying any HTML/CSS/template work, always consult the official
documentation via Context7 MCP **before** writing code. Do not rely on memorized patterns.

| Library          | Context7 ID               | Use for                                      |
|------------------|---------------------------|----------------------------------------------|
| Bootstrap 5      | `/websites/getbootstrap`  | Components, grid, utilities, JS plugins      |
| Bootstrap Icons  | `/twbs/icons`             | Icon names, SVG usage, font usage            |

**When to query docs (mandatory):**
- Implementing any UI component (navbar, modal, offcanvas, accordion, toast, etc.)
- Using grid, flexbox, or spacing utilities
- Checking `data-bs-*` attribute options for JS plugins
- Looking up an icon name before using it in a template
- Verifying correct class names for color, sizing, or state variants

**Workflow:**
1. Call `query-docs` with the relevant Context7 library ID and a specific question.
2. Implement based on the documented pattern — do not guess class names or attributes.
3. After any SCSS change, run `task css` and bump `CACHE_NAME` in `sw.js`.

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
- **All API and web handlers used in router MUST have Swagger annotations** — this ensures API documentation stays current. When adding a new handler to `src/router/router.go`, immediately add the corresponding Swagger comments above the function declaration.

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

## Conventional Commits

This project uses [Conventional Commits](https://www.conventionalcommits.org/) for automated changelog generation via git-cliff.

- **Commit types**: feat, fix, docs, chore, refactor, test, ci, build, style, perf, deprecat, remove, delete, security, implement, add, hotfix, bug
- **Format**: `<type>[(<scope>)]!: <description>` — e.g., `feat: add barcode scanner` or `fix(api): resolve timeout`
- **Breaking changes**: add `!` after type/scope — e.g., `feat!: drop support for SQLite`
- **Body**: optional, separate from subject with blank line

## Release Automation

- **CHANGELOG generation**: When a `v*` tag is pushed, `.forgejo/workflows/release.yml` triggers. It uses `git-cliff` (via `orhun/git-cliff-action@v3`) with `cliff.toml` to generate the CHANGELOG from conventional commits, then commits it back to the repository and attaches it as a release asset.
- **RELEASE_TOKEN secret**: The `RELEASE_TOKEN` secret must exist in the Forgejo repository settings with `api` and `write` scopes. It is used to:
  1. Push the updated `CHANGELOG.md` back to the repository
  2. Upload `CHANGELOG.md` as a release asset via `gitea.com/actions/release-action`
- **Tag format**: Use `vX.Y.Z` tags (e.g., `v0.4.0`). The `cliff.toml` tag pattern `v?[0-9].*` supports both new `v`-prefixed and legacy unprefixed tags.

## Important Notes
- **Always use Context7 MCP** for code generation, setup/configuration steps, and library/API documentation — automatically call `resolve-library-id` and `query-docs` without waiting to be asked explicitly.
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

## UI/UX Standards

### Color & Contrast
- Use CSS custom properties (Bootstrap variables) for theming — no raw hex values in templates
- Maintain WCAG AA 4.5:1 minimum contrast ratio for normal text
- Reserve the primary color for primary CTAs only; use secondary/neutral tones for non-essential actions

### Layout & Spacing
- Align to a 4pt/8pt grid using Bootstrap utility classes (e.g., `p-2`, `m-3`, `gap-2`)
- Ensure touch targets are at least 44×44px for buttons, links, and form controls
- **Never use inline `style="..."` attributes** for layout; define reusable utility or component classes

### Typography
- Limit the project to a maximum of five font sizes to preserve visual consistency
- Establish hierarchy through font weight and size, not color alone

### User Guidance
- For any async operation exceeding 500ms, show a loading indicator and disable the triggering button until completion
- Primary actions must use the `btn-primary` class
- Provide inline validation on-blur for form fields; reserve full-page validation feedback for submit-time results
- Always include a clear call-to-action in empty states (e.g., "Add your first product")
- Require explicit confirmation before irreversible actions (e.g., deletion dialogs)

### Feedback
- Use `proviant.showFeedback()` for page-level async result messages
- Use inline alerts for field-level or section-level errors only; avoid global banners for localized issues

### Accessibility
- Ensure the `lang` attribute on `<html>` matches the active UI language
- All icon-only buttons require an `aria-label` describing the action
- All interactive elements must be keyboard-navigable (`tabindex`, `focus-visible` styles)
- Populate `aria-live` regions after dynamic content updates so screen readers announce changes

### Nielsen 10 Heuristics (Quick Reference)
1. Visibility of system status
2. Match between system and the real world
3. User control and freedom
4. Consistency and standards
5. Error prevention
6. Recognition rather than recall
7. Flexibility and efficiency of use
8. Aesthetic and minimalist design
9. Help users recognize, diagnose, and recover from errors
10. Help and documentation
