# Agent Guidelines for Proviant

Agent coding guidelines for Proviant codebase.

## Code Search

Use `semble search` to find code by description or symbol instead of grep:

```bash
semble search "authentication flow" ./my-project
semble search "save_pretrained" ./my-project
semble search "save model to disk" ./my-project --top-k 10
```

Use `semble find-related` to discover similar code at known location (pass `file_path` and `line` from prior search):

```bash
semble find-related src/auth.py 42 ./my-project
```

`path` defaults to current directory when omitted; git URLs accepted.

If `semble` not on `$PATH`, use `uvx --from "semble[mcp]" semble` instead.

## Documentation

Use Context7 MCP tools automatically (without being asked) for library/API docs, code generation, or config steps for any lib in this project.

## Workflow

1. Start with `semble search` to find relevant chunks.
2. Inspect full files only when returned chunk lacks context.
3. Optionally use `semble find-related` with promising result's `file_path` and `line` to discover related implementations.
4. Use grep only for exhaustive literal matches or exact string confirmation.

## Build, Lint, and Test Commands

**Always prefer `task` wrappers** over vanilla `npm`/`go`/`podman` calls. The Taskfile centralizes flags, paths, and side-effects (cache dirs, font/icon copying, container mounts) so the same task works identically across machines.

### Task cheat sheet

| Task | Use when… |
|------|-----------|
| `task init` | First-time setup or after pulling changes that touch `package.json` / `go.mod`. Installs npm deps, copies vendor assets, builds CSS/JS, runs `go get -u`. |
| `task css` | You edited any `.scss` file under `src/templates/scss/`. SCSS is what gets compiled to `src/assets/css/` — edits have no effect until recompiled. |
| `task js` | You added/changed a frontend npm dep (bootstrap, chart.js, html5-qrcode) that must be copied into `src/assets/js/`. Pure-JS source edits do not need this. |
| `task icons` | You added/updated files under `res/icons/` and need them propagated to `src/assets/icons/`. |
| `task fonts` | Font files changed. Usually called automatically as a dep of `task css`. |
| `task lint` / `task lint-css` / `task lint-js` / `task lint-html` | Before committing frontend changes. Runs stylelint/eslint/htmlhint. |
| `task vet` | Quick `go vet` pass without running the full linter container. |
| `task tidy` | Before committing Go changes. Runs `go fmt` and `go mod tidy`. |
| `task test` | Before committing. Runs the full Go test suite with `-race`. |
| `task test-single` | Running a single test — use the pattern in "Quality Control" below, or invoke `go test` directly inside `src/`. |
| `task check` | Pre-commit / pre-push Go lint via golangci-lint in a container. Replaces raw `golangci-lint run`. |
| `task check-changed` | Lint only files changed vs. previous commit. Faster local feedback loop. |
| `task doc` | After adding/changing exported Go symbols or Swagger annotations. Regenerates godoc + Swagger output. |
| `task run` | Local dev server. |
| `task containerimage` / `task runcontainer` / `task runcontainerdebug` | Building or running the containerized app. |
| `task vuln` / `task vuln-go` / `task vuln-npm` | Security audits for Go and npm deps. |
| `task vuln-image` | Scan the container image (requires podman or docker). Not part of `task vuln` on purpose — it forces a full image build. |
| `task seed` / `task seed-reset` | Populate / wipe demo products for a user. |

### Do not
- Do not run `npm install`, `npm run css`, `npm run lint-*`, or `npm test` directly — use `task init` / `task css` / `task lint` / `task test` (or a direct `go test` for the single-test case below) so deps, caches, and asset copies stay in sync.
- Do not run `podman`/`docker build` directly — use `task containerimage`.
- Do not run `golangci-lint run` directly — use `task check` (handles container, config mount, cache dirs).
- Do not run `go run` for the dev server — use `task run`.

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

# Lint all frontend
task lint

# Lint a single domain
task lint-css
task lint-js
task lint-html
```

## Dependency Auditing

`npm audit` gates on **production dependencies only** (`--omit=dev`). A second, non-blocking run reports dev-dependency findings for visibility without failing the build.

The policy lives in exactly one place — `scripts/npm-audit.sh`. Both CI workflows invoke it through the composite action `.github/actions/npm-audit`, and `task vuln-npm` calls it directly, so the gate cannot drift between CI and local runs. Do not inline `npm audit` into a workflow or Taskfile; edit the script instead.

**Why:** advisory GHSA-vfj7-8cjw-p6xm (CVE-2026-93687) affects `braces` through `3.0.3`, and `3.0.3` is the latest published version — there is no fix, so `npm audit fix` can never resolve it. It reaches the tree only through `stylelint` (a devDependency) → `micromatch` / `fast-glob` / `globby`. Upgrading `stylelint` does not help: current `stylelint@17.16.0` still depends on `micromatch ^4.0.8`. None of these packages ship; they execute only during `task lint-css` with repo-controlled glob arguments, so no untrusted input reaches `braces` and the DoS precondition does not exist here. `package-lock.json` marks `node_modules/braces` and `node_modules/micromatch` with `"dev": true`.

**Re-check trigger:** when a `braces` release greater than `3.0.3` ships, drop `--omit=dev` from `scripts/npm-audit.sh` and re-enable the full gate. Track upstream: https://github.com/micromatch/braces/issues/70

Do not reintroduce `npm audit fix --force` or a `braces` `overrides` entry — no fixed version exists, so neither clears the advisory.

## Container Image Scanning

`trivy` scans the production image (`Dockerfile`, `alpine:3.23` runtime layer, incl. tesseract OS packages — invisible to `govulncheck`/`npm audit`). Policy lives in exactly one place — `scripts/image-scan.sh`. It is called by `.github/workflows/ci.yml` (`image-scan` job: build and push the prod image to a `ci-scan-<sha>` tag, scan it, then `docker-build-and-push` promotes **that exact digest** to the prod tags — a rebuild between gate and push would change the digest, so the report must cover what ships), `.github/workflows/release.yml` (`docker-publish`: same promote-by-digest shape against the `:scan` tag) and `.github/workflows/weekly-vuln-scan.yml` (`image-scan` job, build-without-push then scan, so new base-image CVEs surface weekly instead of only on the next push to `main`). `task vuln-image` calls it locally. Do not inline `trivy` flags into a workflow or the Taskfile; edit the script instead.

**Gate:** `--scanners vuln` + `--severity HIGH,CRITICAL` + `--ignore-unfixed` + `--exit-code 1`, report to `trivy-report.json` (uploaded as a CI artifact). `--scanners vuln` is explicit because `trivy image` defaults to `vuln,secret` and a HIGH-severity secret finding would otherwise fail a gate that is scoped to OS packages. Unfixed findings never fail the build — no maintainer action exists for them; fix them by bumping the base image, never by silencing the scanner.

**Failure classification:** trivy exits non-zero both for findings and for operational failures (DB download, unreadable image, killed container), and a partially written report looks like a clean one — so **neither signal is trusted alone.** `classify()` in `scripts/image-scan.sh` combines them three ways: non-zero exit **and** an empty/absent `Results` array = infrastructure error; non-zero exit with findings = vulnerability verdict; zero exit with zero findings = clean. The rc!=0-and-zero-findings case is the one that must never pass: reporting an aborted scan as "no vulnerabilities found" turns a scanner outage into a green gate on `main` and on every release. Keep the report on every exit path; it is the artifact a maintainer needs most when the gate goes red.

**Scanner provenance:** the local-binary path is not digest-pinned, so the script records `trivy --version` into `$GITHUB_STEP_SUMMARY` when that variable is set. Do not remove it: a runner image that stops shipping trivy silently flips CI to the container path, and without the summary nothing in the workflow record shows which scanner actually ran.

**Trivy source / digest pin:** the script prefers a local `trivy` binary (GitHub runners ship one — CI always runs that path); otherwise it runs the digest-pinned container image from `$TRIVY_IMAGE` (default in the script, `TRIVY_IMAGE` var in the Taskfile). **Digest, not version tag:** the March 2026 trivy supply-chain compromise (GHSA-69fq-xp46-6x23 — Docker Hub tags `0.69.4`–`0.69.6` and `latest` poisoned, `trivy-action`/`setup-trivy` tags rewritten) proved Docker Hub tags are mutable and cannot serve as an integrity anchor. Current pin: `docker.io/aquasec/trivy@sha256:af6acf9a…` (v0.75.0, post-remediation release). **Bump procedure:** fetch the digest for the target version (`curl -s https://hub.docker.com/v2/repositories/aquasec/trivy/tags/<VER>` or `podman image inspect`) — note the tags are **unprefixed** (`0.75.0`, not `v0.75.0`; the `v` form 404s), confirm the release postdates the incident remediation per the upstream advisory, update `scripts/image-scan.sh` (default) **and** the Taskfile `TRIVY_IMAGE` var together, verify with `task vuln-image`.

**Container scanning detail:** the container path scans a `docker/podman save` tarball (the Taskfile exports one) because the trivy container has no host runtime socket; full registry-qualified names are required or podman fails with "short-name resolution enforced". Vulnerability DB caches under `.cache/trivy/` (already gitignored via `.cache/`) — `$TRIVY_CACHE_DIR` is honored by both paths (passed to the local binary via `--cache-dir`, bind-mounted in the container path) and cached in every CI workflow with `actions/cache`. **The cache key ends in `${{ github.run_id }}` with a `trivy-db-<os>-` restore prefix, and must keep doing so:** a constant key makes `actions/cache` skip every save after the first, which pins the DB to whatever day one shipped and re-downloads ~120 MB on every run. Do not add `--format table` alongside `--format json --output` — trivy binds `-o` to one format and the table would clobber the report.

**Promotion must stay digest-based:** `docker-build-and-push` and `release.yml` resolve the scanned index digest and copy it with `docker buildx imagetools create`. Do not "simplify" this to `docker pull` + `docker tag` + `docker push` — a daemon-loaded image carries neither the attestations nor any link to the report that vouched for it.

**Scan tag lifecycle:** `image-scan` publishes an immutable `ci-scan-<sha>` tag (the promotion source) plus a rolling `ci-scan-latest` on `main`/`develop` only, so a PR run cannot hijack the pointer. `cleanup-scan-tags` prunes `ci-scan-*` older than 7 days via the GHCR API. Do not drop the rolling tag's ref guard, and do not lower the retention below the worst realistic CI duration.

**CI trigger note:** `ci.yml` runs on `push` **and** `pull_request` (branches `main`/`develop`). Consequences: `sonarcloud-scan`'s pre-existing `github.event_name == 'pull_request'` condition is now live; `docker-build-and-push` stays push-only (its `github.ref` guard excludes `refs/pull/*/merge`); `check`'s "Create vulnerability issues" step is guarded with `github.event_name == 'push'` because `GITHUB_TOKEN` is read-only on fork PRs (unguarded curl = 403 = red CI through no fault of the contributor); and `image-scan` is guarded with a same-repo check, because it pushes to GHCR and a fork PR's token is read-only. **Fork PRs therefore run no image scan** — the gate is enforced by branch protection on `main`/`develop`, not by PR CI, so do not describe it as gating every pull request. `image-scan` deliberately does *not* depend on `frontend-check`: the security gate and the prod image must not be blocked by an unrelated CSS/JS/HTML lint failure.

**Re-check trigger:** when the first real scan produces false positives or new unfixable base-image CVEs, tune the ignore list *inside* `scripts/image-scan.sh` with a comment + upstream link (same treatment as the `braces` advisory above). Also re-check this pin whenever a new trivy release or new supply-chain advisory lands.

## Code Style Guidelines

### Imports
- Group imports: stdlib → internal packages (codeberg.org) → external packages
- Use blank identifier `_` for side-effect imports (e.g., `image/jpeg`), add comment explaining why
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
- Tabs for indentation (Go standard)
- Max line length: ~120 chars (recommended, not enforced)

### Naming Conventions
- **Exported functions/types**: PascalCase (e.g., `GetProduct`, `DatabaseController`)
- **Unexported functions/types**: camelCase (e.g., `setupConfig`, `handleError`)
- **Constants**: PascalCase descriptive (e.g., `PreferredTimeFormat`, `GeneratedPrefix`)
- **Package names**: lowercase single words (e.g., `database`, `router`)
- **Interfaces**: PascalCase, often functional description suffix (e.g., `Authorizator`)
- **Error variables**: PascalCase with `Err` prefix (e.g., `ErrInvalidUserID`, `ErrDatabaseInvalidEngine`)
- **Variables**: Full descriptive camelCase — never abbreviate (e.g., `storageLocationRepo` not `slRepo`, `productController` not `pc`, `householdID` not `hID`)

### Types and Structs
- GORM struct tags for DB models: `gorm:"index, not null"`
- JSON struct tags for API responses: `json:"fieldName"`
- `json:"-"` to exclude sensitive fields from JSON
- EAN-13 format for barcode (alphanumeric, 13 digits)
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
- Use predefined errors from `errors` package, not generic strings
- Return errors from functions (no `panic` in normal flow)
- **Use early returns instead of nested if-clauses** — check error conditions first, return immediately, keep main logic at base indentation
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
- Use GORM for all DB queries
- Use transactions for multi-step operations: `tx := db.Begin()` → `tx.Commit()` or `tx.Rollback()`
- Soft delete via GORM's `DeletedAt` field (use `.Unscoped()` for hard delete)
- Check record not found: `if err.Error != gorm.ErrRecordNotFound`
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
- Log levels: `Debug()`, `Info()`, `Warn()`, `Error()`, `Fatal()`
- Format with `Msg()` or `Msgf()`
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
| **Warn** | Recoverable, expected, or user-caused issues not requiring immediate action | Invalid user input (4xx), missing optional dependencies, background task failures, cache misses/errors, "not found" in user-requested lookups, file close errors |
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
- Use `testutil.SetupTestDB(t)` for DB setup — creates in-memory SQLite with all models migrated
- Use `testutil.SetupGinContext(db)` for API handler tests — creates test context with logger and db
- Use `t.Run()` for all subtests

#### Test Helpers (`src/testutil/`)
- `testutil.SetupTestDB(t)` — in-memory SQLite DB with all migrations
- `testutil.SetupGinContext(db)` — Gin test context with logger and db
- `testutil.MockJWTClaims(ctx, userID)` — JWT claims with "id" key
- `testutil.MockJWTClaimsWithKey(ctx, userID, key)` — JWT claims with custom key
- `testutil.CreateTestUser(db, householdID)` — creates test user
- `testutil.CreateTestHousehold(db, adminID)` — creates test household
- `testutil.CreateTestProduct(db, householdID)` — creates test product
- `testutil.CreateTestStorageLocation(db, householdID)` — creates test storage location

#### Assertions
- Plain Go assertions: `if got != want { t.Errorf(...) }`
- Both plain Go and testify/assert acceptable; prefer plain Go for consistency

#### Test Guidelines
- Use `go test` for unit tests
- Use `t.Run()` for subtests
- Test both success and error paths
- Keep test DB setup in helper functions, not duplicated per test file

### Frontend/Assets
- SCSS files in `src/templates/scss/`
- Compiled CSS goes to `src/assets/css/`
- **Always run `task css` after any `.scss` change** — compiled CSS is what gets served; SCSS edits without recompile have no effect
- JS files served from `src/assets/js/`
- **Always use Bootstrap 5 for styling/layout** — Context7 library ID: `/websites/getbootstrap`
- **Always use Bootstrap Icons for icons** — Context7 library ID: `/twbs/icons`
- **No inline `style="..."` attributes in templates** — define CSS classes in `src/templates/scss/main.scss`. Inline styles acceptable only for truly dynamic values (e.g., `width` from template variable). When in doubt, use a class.

#### Bootstrap & Bootstrap Icons — Documentation Workflow
Before any HTML/CSS/template work, consult official docs via Context7 MCP. Do not rely on memorized patterns.

| Library          | Context7 ID               | Use for                                      |
|------------------|---------------------------|----------------------------------------------|
| Bootstrap 5      | `/websites/getbootstrap`  | Components, grid, utilities, JS plugins      |
| Bootstrap Icons  | `/twbs/icons`             | Icon names, SVG usage, font usage            |

**Query docs (mandatory):**
- Any UI component (navbar, modal, offcanvas, accordion, toast, etc.)
- Grid, flexbox, or spacing utilities
- `data-bs-*` attribute options for JS plugins
- Icon name lookup before use in template
- Correct class names for color, sizing, or state variants

**Workflow:**
1. Call `query-docs` with relevant Context7 library ID and specific question.
2. Implement from documented pattern — do not guess class names or attributes.
3. After any SCSS change, run `task css` and bump `CACHE_NAME` in `sw.js`.

#### Service Worker Caching
- Project uses service worker (`src/assets/js/sw.js`)
- **CSS and non-JS assets** under `/assets/` use **cache-first** — bump `CACHE_NAME` in `sw.js` after every `task css` run or change to fonts/icons/static assets
- **JS files** use **network-first** — always fetched fresh; bumping `CACHE_NAME` NOT needed for JS changes
- After bumping `CACHE_NAME`, run `task css` (or `task js`) to deploy new version alongside asset change

#### Frontend JavaScript Event Handling
- **Always use event delegation**: `document.addEventListener("click", ...)` with `event.target.closest()`
- **Never capture DOM references at script load time** (e.g., `const btn = document.getElementById("btnX"); btn.onclick = ...`) — become stale after navigation or DOM updates
- Query DOM elements inside handler, not at module scope
- Ensures handlers work regardless of browser caching or SPA-like navigation
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
- godoc comments for exported functions
- Swagger annotations for API endpoints
- Run `task doc` to regenerate docs before commits
- Swagger format: `@Summary`, `@Description`, `@Tags`, `@Router`
- **All API and web handlers in router MUST have Swagger annotations** — when adding handler to `src/router/router.go`, immediately add Swagger comments above function declaration.

### Performance Considerations
- Use `strings.Builder` instead of string concatenation in loops
- Use `strings.SplitSeq` for splitting (Go 1.18+)
- Pre-calculate buffer sizes for `strings.Builder` when known
- Avoid `fmt.Sprintf` for simple string building in hot paths

## Repository Structure
- `src/api/` — API endpoints and handlers
- `src/assets/` — Static assets (CSS, JS, fonts, icons)
- `src/controllers/` — Business logic controllers
- `src/errors/` — Custom error definitions
- `src/models/` — Data models (database, API, authentication, configuration)
- `src/router/` — Gin router setup and middleware
- `src/templates/` — HTML templates with embedded assets
- `src/web/` — Frontend page handlers
- `docs/` — Generated documentation

## Dashboard / Portal

Home page (`/web/`) is fully client-side rendered. Go handler (`web/frontend.go → Root`) provides only `HasHousehold` and `Household` — no server-side tile data.

**API endpoint:** `GET /api/v1/products/stats` (JWT-protected) — returns `apiModel.ProductStatsResponse` defined in `src/models/api/stats.go`. Fields:
- `wasteCount`, `wastePercent`, `totalActive` — active/expired counts
- `totalArchived`, `uniqueArchived` — archived product counts
- `lastInsertedProduct` — name of most recently added product
- `expiringSoon` — `[]StatsExpiringProduct` (name + date) for products expiring within 7 days (today inclusive), sorted ascending
- `categories` — `map[string]int` category breakdown of active products
- `expiryTrend` — `[]StatsMonthlyCount` for next 12 calendar months

**Frontend:** `src/assets/js/homeStats.js` fetches stats endpoint on `DOMContentLoaded` and:
1. Prepends metric tiles (via `renderTile` / `renderListTile`) into `#dashboard` before `.chart-col` sentinel elements
2. Renders three Chart.js charts into canvas elements in `home.tmpl`

**Chart.js:** bundled as `src/assets/js/chart.umd.min.js` (copied from `node_modules` via `task js`). Loaded via `<script>` tag in `home.tmpl` before `homeStats.js`.

**Removed:** `GetUserHomeTiles` DB method and `webparts` import from `databasecontroller.go` gone — tile data now comes entirely from stats API.

## Conventional Commits

Project uses [Conventional Commits](https://www.conventionalcommits.org/) for automated changelog generation via git-cliff.

- **Commit types**: feat, fix, docs, chore, refactor, test, ci, build, style, perf, deprecat, remove, delete, security, implement, add, hotfix, bug
- **Format**: `<type>[(<scope>)]!: <description>` — e.g., `feat: add barcode scanner` or `fix(api): resolve timeout`
- **Breaking changes**: add `!` after type/scope — e.g., `feat!: drop support for SQLite`
- **Body**: optional, separate from subject with blank line

## Release Automation

- **CHANGELOG generation**: On `v*` tag push, `.github/workflows/release.yml` triggers. Uses `git-cliff` (via `taiki-e/install-action@git-cliff`) with `cliff.toml` to generate CHANGELOG from conventional commits, commits it back, attaches as release asset.
- **Auth**: Uses the built-in `GITHUB_TOKEN` (with `permissions: contents: write`). `RELEASE_TOKEN` secret is no longer required. The push-back to `main` uses the `x-access-token` URL form (`https://x-access-token:${{ secrets.GITHUB_TOKEN }}@github.com/...`). Note: branch protection requiring PR reviews will block the push-back — either exempt the release bot or allow direct pushes for releases.
- **Release creation**: Uses `softprops/action-gh-release@v2` to create the GitHub Release and upload `CHANGELOG.md` as an asset.
- **Tag format**: `vX.Y.Z` tags (e.g., `v0.4.0`). `cliff.toml` tag pattern `v?[0-9].*` supports both `v`-prefixed and legacy unprefixed tags.

## Important Notes
- **NEVER commit any change without explicit user confirmation.** Propose the commit message, then wait for the user to approve. This applies to every `git commit` (and `git push`), regardless of task size or prior instructions.
- **Always use Context7 MCP** for code generation, setup/configuration, library/API docs — auto-call `resolve-library-id` and `query-docs` without waiting to be asked.
- Run `task check` before committing — golangci-lint must pass
- **Database migrations**: When creating or modifying DB model, always add to `AutoMigrate` call in `src/proviant.go` (~line 188). Omitting causes "no such table" at runtime. Example:
  ```go
  migrationError := dbHandle.AutoMigrate(
      &dbModel.Household{},
      &authentication.User{},
      &dbModel.Product{},
      &dbModel.HouseholdApplication{},
      &dbModel.YourNewModel{},  // ← always add new models here
  )
  ```
- **OpenFoodFacts caching**: Barcode lookups go through backend proxy `GET /api/v1/products/openfoodfacts/:barcode` (JWT-protected). When `openfoodfacts.cacheEnabled: true`, responses stored in `open_food_facts_caches` table (`src/models/database/openfoodfacts_cache.go`) and served from there on subsequent requests. Frontend (`src/assets/js/productsCreate.js`) calls `proviant.getOpenFoodFactsData()` from `proviant.js` — **do not** reintroduce direct browser calls to `world.openfoodfacts.org`. Cache operations in `src/controllers/database/databasecontroller.go` (`GetOpenFoodFactsCacheByBarcode`, `CreateOpenFoodFactsCache`).
- **Config values**: New config blocks (e.g., `mailDigest`, `ntfy`, `telegram`) go under `notification:` section in config files. Always add to all three: `config.yaml`, `config.yaml.sqlite.tmpl`, `config.yaml.mariadb.tmpl`.
- **String literals**: New string constants (route labels, config keys, section identifiers) must be centralized in `src/util/util.go` under a appropriate group comment.

## UI/UX Standards

### Color & Contrast
- Use CSS custom properties (Bootstrap variables) for theming — no raw hex values in templates
- Maintain WCAG AA 4.5:1 minimum contrast ratio for normal text
- Reserve primary color for primary CTAs only; use secondary/neutral tones for non-essential actions

### Layout & Spacing
- Align to 4pt/8pt grid using Bootstrap utility classes (e.g., `p-2`, `m-3`, `gap-2`)
- Touch targets min 44×44px for buttons, links, form controls
- **No inline `style="..."` attributes** for layout; define reusable utility or component classes

### Typography
- Max five font sizes to preserve visual consistency
- Hierarchy via font weight and size, not color alone

### User Guidance
- Async operations exceeding 500ms: show loading indicator and disable triggering button until completion
- Primary actions must use `btn-primary` class
- Inline validation on-blur for form fields; full-page validation feedback only at submit time
- Empty states always include clear call-to-action (e.g., "Add your first product")
- Require explicit confirmation before irreversible actions (e.g., deletion dialogs)

### Feedback
- Use `proviant.showFeedback()` for page-level async result messages
- Inline alerts for field-level or section-level errors only; no global banners for localized issues

### Accessibility
- `lang` attribute on `<html>` must match active UI language
- Icon-only buttons require `aria-label` describing action
- All interactive elements must be keyboard-navigable (`tabindex`, `focus-visible` styles)
- Populate `aria-live` regions after dynamic content updates for screen reader announcements

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
