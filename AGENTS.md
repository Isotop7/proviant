# Agent Guidelines for Proviant

Agent coding guidelines for Proviant codebase.

## Code Search

Find code by description or symbol via `semble search`, not grep:

```bash
semble search "authentication flow" ./my-project
semble search "save_pretrained" ./my-project
semble search "save model to disk" ./my-project --top-k 10
```

Similar code at known location via `semble find-related` (pass `file_path` + `line` from prior search):

```bash
semble find-related src/auth.py 42 ./my-project
```

`path` defaults to current dir, git URLs accepted. If `semble` not on `$PATH`: `uvx --from "semble[mcp]==0.6.1" semble`.

## Documentation

Use Context7 MCP tools automatically (no asking) for library/API docs, code generation, config steps for any lib in project.

## Workflow

1. Start with `semble search`.
2. Read full files only when chunk lacks context.
3. Optional: `semble find-related` on promising result's `file_path` + `line`.
4. Grep only for exhaustive literal matches / exact string confirmation.

## Build, Lint, and Test Commands

**Prefer `task` wrappers** over vanilla `npm`/`go`/`podman`. Taskfile centralizes flags, paths, side-effects (cache dirs, font/icon copying, container mounts) — same task works across machines.

### Task cheat sheet

| Task | Use when… |
|------|-----------|
| `task init` | First-time setup or after `package.json`/`go.mod` changes. npm deps, vendor assets, CSS/JS build, `go get -u`. |
| `task css` | `.scss` under `src/templates/scss/` edited. SCSS compiles to `src/assets/css/` — no effect until recompiled. |
| `task js` | New/changed frontend npm dep (bootstrap, chart.js, html5-qrcode) copied into `src/assets/js/`. Pure-JS source edits skip. |
| `task icons` | Files under `res/icons/` added/updated → propagate to `src/assets/icons/`. |
| `task fonts` | Font files changed. Auto-called as dep of `task css`. |
| `task lint` / `task lint-css` / `task lint-js` / `task lint-html` | Before frontend commits. stylelint/eslint/htmlhint. |
| `task vet` | Quick `go vet` without full linter container. |
| `task tidy` | Before Go commits. `go fmt` + `go mod tidy`. |
| `task test` | Before commits. Full Go suite `-race`. Go only — see "Frontend Tests" for JS. |
| `task test-js` | Before frontend commits. Vitest suite in `tests/js/`. |
| `task test-single` | Single test — pattern in "Quality Control" below, or direct `go test` inside `src/`. |
| `task check` | Pre-commit/pre-push Go lint via golangci-lint container. Replaces raw `golangci-lint run`. |
| `task check-changed` | Lint only files changed vs previous commit. Faster feedback. |
| `task doc` | After adding/changing exported Go symbols or Swagger annotations. Regenerates godoc + Swagger. |
| `task run` | Local dev server. |
| `task containerimage` / `task runcontainer` / `task runcontainerdebug` | Build/run containerized app. |
| `task vuln` / `task vuln-go` / `task vuln-npm` | Security audits for Go + npm deps. |
| `task vuln-image` | Scan container image (needs podman/docker). Excluded from `task vuln` — forces full image build. |
| `task seed` / `task seed-reset` | Populate / wipe demo products for user. |

### Do not
- No direct `npm install`, `npm run css`, `npm run lint-*`, `npm test` — use `task init` / `task css` / `task lint` / `task test` (single-test case: direct `go test` below) so deps, caches, asset copies stay in sync. Same for `npm run test-js` → `task test-js`.
- No direct `podman`/`docker build` → `task containerimage`.
- No direct `golangci-lint run` → `task check` (handles container, config mount, cache dirs).
- No `go run` for dev server → `task run`.

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

`npm audit` gates on **production deps only** (`--omit=dev`). Second, non-blocking run reports dev-dep findings without failing build.

Policy lives once — `scripts/npm-audit.sh`. Both CI workflows call it via composite action `.github/actions/npm-audit`; `task vuln-npm` calls directly. Gate cannot drift CI↔local. Never inline `npm audit` into workflow/Taskfile; edit script.

**Why:** advisory GHSA-vfj7-8cjw-p6xm (CVE-2026-93687) affects `braces` through 3.0.3; 3.0.3 latest — no fix, `npm audit fix` can never resolve. Reaches tree via `stylelint` (devDependency) → `micromatch` / `fast-glob` / `globby`. Upgrading `stylelint` doesn't help: `stylelint@17.16.0` still depends on `micromatch ^4.0.8`. None ship; execute only during `task lint-css` with repo-controlled globs — no untrusted input reaches `braces`, DoS precondition absent. `package-lock.json` marks `node_modules/braces` + `node_modules/micromatch` `"dev": true`.

**Re-check trigger:** when `braces` > 3.0.3 ships, drop `--omit=dev` from `scripts/npm-audit.sh`, re-enable full gate. Upstream: https://github.com/micromatch/braces/issues/70

Never reintroduce `npm audit fix --force` or `braces` `overrides` entry — no fixed version exists, neither clears advisory.

## Container Image Scanning

`trivy` scans prod image (`Dockerfile`, `alpine:3.23` runtime layer incl. tesseract OS packages — invisible to `govulncheck`/`npm audit`). Policy lives once — `scripts/image-scan.sh`. Called by `.github/workflows/ci.yml` (`image-scan` job: build+push prod image to `ci-scan-<sha>` tag, scan, then `docker-build-and-push` promotes **that exact digest** — rebuild between gate/push changes digest, report must cover what ships), `.github/workflows/release.yml` (`docker-publish`: same promote-by-digest against `:scan` tag), `.github/workflows/weekly-vuln-scan.yml` (`image-scan` job: build-without-push then scan — new base-image CVEs surface weekly, not only next push to `main`). `task vuln-image` local. Never inline `trivy` flags into workflow/Taskfile; edit script.

**Gate:** `--scanners vuln` + `--severity HIGH,CRITICAL` + `--ignore-unfixed` + `--exit-code 1`, report `trivy-report.json` (CI artifact). `--scanners vuln` explicit: `trivy image` defaults `vuln,secret`, HIGH secret finding would fail OS-packages-scoped gate. Unfixed findings never fail build — no maintainer action exists; fix via base-image bump, never scanner silencing.

**Failure classification:** trivy exits non-zero for findings + operational failures (DB download, unreadable image, killed container); partial report looks clean — **neither signal alone trusted.** `classify()` in `scripts/image-scan.sh` three ways: non-zero exit **and** empty/absent `Results` = infra error; non-zero + findings = vulnerability verdict; zero + zero findings = clean. rc≠0-and-zero-findings must never pass: reporting aborted scan as "no vulnerabilities found" turns scanner outage into green gate on `main` + every release. Keep report on every exit path — artifact maintainer needs most when gate red.

**Scanner provenance:** local-binary path not digest-pinned, script records `trivy --version` into `$GITHUB_STEP_SUMMARY` when set. Don't remove: runner image dropping trivy silently flips CI to container path; without summary nothing shows which scanner ran.

**Trivy source / digest pin:** script prefers local `trivy` binary (GitHub runners ship one — CI always takes path); else digest-pinned container from `$TRIVY_IMAGE` (script default, `TRIVY_IMAGE` var in Taskfile). **Digest, not version tag:** March 2026 trivy supply-chain compromise (GHSA-69fq-xp46-6x23 — Docker Hub tags `0.69.4`–`0.69.6` + `latest` poisoned, `trivy-action`/`setup-trivy` tags rewritten) proved Docker Hub tags mutable, no integrity anchor. Current pin: `docker.io/aquasec/trivy@sha256:af6acf9a…` (v0.75.0, post-remediation). **Bump procedure:** fetch target digest (`curl -s https://hub.docker.com/v2/repositories/aquasec/trivy/tags/<VER>` or `podman image inspect`) — tags **unprefixed** (`0.75.0`, not `v0.75.0`; `v` form 404s), confirm release postdates remediation per upstream advisory, update `scripts/image-scan.sh` (default) **and** Taskfile `TRIVY_IMAGE` together, verify `task vuln-image`.

**Container scanning detail:** container path scans `docker/podman save` tarball (Taskfile exports) — trivy container has no host runtime socket; full registry-qualified names required or podman fails "short-name resolution enforced". Vuln DB caches under `.cache/trivy/` (gitignored via `.cache/`) — `$TRIVY_CACHE_DIR` honored both paths (local binary `--cache-dir`, bind-mounted container path), cached in every CI workflow via `actions/cache`. **Cache key must end `${{ github.run_id }}` with `trivy-db-<os>-` restore prefix:** constant key makes `actions/cache` skip saves after first — pins DB to day one, re-downloads ~120 MB per run. Never add `--format table` alongside `--format json --output` — trivy binds `-o` to one format, table clobbers report.

**Promotion stays digest-based:** `docker-build-and-push` + `release.yml` resolve scanned index digest, copy via `docker buildx imagetools create`. Never "simplify" to `docker pull` + `docker tag` + `docker push` — daemon-loaded image carries neither attestations nor link to vouching report.

**Scan tag lifecycle:** `image-scan` publishes immutable `ci-scan-<sha>` (promotion source) + rolling `ci-scan-latest` on `main`/`develop` only — PR run can't hijack pointer. `cleanup-scan-tags` prunes `ci-scan-*` older 7 days via GHCR API. Don't drop rolling tag's ref guard, don't lower retention below worst realistic CI duration.

**CI trigger note:** `ci.yml` runs `push` **and** `pull_request` (`main`/`develop`). Consequences: `sonarcloud-scan`'s pre-existing `github.event_name == 'pull_request'` condition now live; `docker-build-and-push` push-only (`github.ref` guard excludes `refs/pull/*/merge`); `check`'s "Create vulnerability issues" step guarded `github.event_name == 'push'` — `GITHUB_TOKEN` read-only on fork PRs (unguarded curl = 403 = red CI, contributor faultless); `image-scan` guarded same-repo check — pushes GHCR, fork PR token read-only. **Fork PRs run no image scan** — gate enforced by branch protection on `main`/`develop`, not PR CI; don't describe as gating every PR. `image-scan` deliberately no `frontend-check` dep: security gate + prod image must not block on unrelated CSS/JS/HTML lint failure.

**Re-check trigger:** first real scan false positives or new unfixable base-image CVEs → tune ignore list *inside* `scripts/image-scan.sh` with comment + upstream link (same as `braces` treatment). Also re-check pin on every new trivy release or supply-chain advisory.

## Code Style Guidelines

### Imports
- Group: stdlib → internal (codeberg.org) → external
- Blank identifier `_` for side-effect imports (e.g., `image/jpeg`), comment why
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
- `go fmt` (`task tidy` before commits)
- Tabs (Go standard)
- Max ~120 chars (recommended, unenforced)

### Comments
- **Allowed / required:**
  - godoc on exported symbols (see `### Documentation`)
  - Swagger annotations on handlers (see `### Documentation`)
  - One doc comment above a test function stating its intent
  - One line on a side-effect (`_`) import saying why (tooling convention)
- **Discouraged:** everything else — comments restating what the code does, narrating steps, or explaining a workaround/hack. Rename the function/variable instead, or move the intent into the test name. No exception with a revisit trigger: if the code needs a paragraph, the code is wrong.
- Applies to `_test.go` too: doc comment *above* the test, none inside restating assertions.

```go
// WRONG: 4-line rationale paragraph explaining obvious code
// A client-side probe timeout cancels the request context: the caller
// gave up, which is not the database failing. Logging it at Error would
// turn an ordinary 1s kubelet timeout into a self-inflicted outage
// signal from a process whose database is merely slow.
logProbeOutcome(ctx, err)

// CORRECT: the name carries it, no comment
logClientAbandonedProbe(ctx, err)
```

### Naming Conventions
- **Exported functions/types**: PascalCase (`GetProduct`, `DatabaseController`)
- **Unexported functions/types**: camelCase (`setupConfig`, `handleError`)
- **Constants**: PascalCase descriptive (`PreferredTimeFormat`, `GeneratedPrefix`)
- **Package names**: lowercase single words (`database`, `router`)
- **Interfaces**: PascalCase, functional description suffix (`Authorizator`)
- **Error variables**: PascalCase `Err` prefix (`ErrInvalidUserID`, `ErrDatabaseInvalidEngine`)
- **Variables**: full descriptive camelCase — never abbreviate (`storageLocationRepo` not `slRepo`, `productController` not `pc`, `householdID` not `hID`)

### Types and Structs
- GORM tags: `gorm:"index, not null"`
- JSON tags: `json:"fieldName"`
- `json:"-"` excludes sensitive fields from JSON
- EAN-13 barcode (alphanumeric, 13 digits)
- Embed `gorm.Model` → ID, CreatedAt, UpdatedAt, DeletedAt
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
- Custom errors as package vars in `src/errors/errors.go`
- Use `errors` package predefined, not generic strings
- Return errors (no `panic` in normal flow)
- **Early returns over nested if-clauses** — error checks first, immediate return, main logic at base indentation
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
- GORM for all queries
- Transactions for multi-step: `tx := db.Begin()` → `tx.Commit()` / `tx.Rollback()`
- Soft delete via `DeletedAt` (`.Unscoped()` hard delete)
- Not-found: `if err.Error != gorm.ErrRecordNotFound`
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
- Logger: `logger, _ := ctx.MustGet("logger").(*zerolog.Logger)`
- DB handle: `dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)`
- JWT claims: `claims := jwt.ExtractClaims(ctx)`
- Proper HTTP codes: `ctx.JSON(http.StatusBadRequest, api.APIResponse{...})`
- `ctx.MustGet` for required context values, handle `!ok` return

### Logging
- zerolog, logger from Gin context (don't create new)
- Levels: `Debug()`, `Info()`, `Warn()`, `Error()`, `Fatal()`
- Format: `Msg()` / `Msgf()`
- Example:
```go
logger, _ := ctx.MustGet("logger").(*zerolog.Logger)
logger.Warn().Msg(err.Error())
logger.Info().Msg("Logging initialized")
```

#### Log Level Policy

| Level | When to use | Examples |
|-------|-------------|----------|
| **Trace** | SQL tracing, extreme debug detail | GORM adapter SQL tracing |
| **Debug** | Dev-time info, cache hit/miss, migration steps, event processing | "migration step running", "cache hit", "recipe match attempt" |
| **Info** | Normal production-worthy operational events | Startup, user created, config loaded, cleanup counts, notification/email sent |
| **Warn** | Recoverable, expected, user-caused — no immediate action | Invalid input (4xx), missing optional deps, background task failures, cache misses/errors, user-requested "not found", file close errors |
| **Error** | Unexpected server-side failure needing attention | DB op failures, external API failures, missing required context (controllers, repos), 5xx errors, internal logic errors |
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
- `testutil.SetupTestDB(t)` — in-memory SQLite, all models migrated
- `testutil.SetupGinContext(db)` — API handler test context w/ logger + db
- `t.Run()` all subtests

#### Test Helpers (`src/testutil/`)
- `testutil.SetupTestDB(t)` — in-memory SQLite, all migrations
- `testutil.SetupGinContext(db)` — Gin test context, logger + db
- `testutil.MockJWTClaims(ctx, userID)` — JWT claims, "id" key
- `testutil.MockJWTClaimsWithKey(ctx, userID, key)` — custom key
- `testutil.CreateTestUser(db, householdID)` — test user
- `testutil.CreateTestHousehold(db, adminID)` — test household
- `testutil.CreateTestProduct(db, householdID)` — test product
- `testutil.CreateTestStorageLocation(db, householdID)` — test storage location

#### Assertions
- Plain Go: `if got != want { t.Errorf(...) }`
- Plain Go + testify/assert both OK; prefer plain Go

#### Test Guidelines
- `go test` unit tests
- `t.Run()` subtests
- Test success + error paths
- Test DB setup in helpers, not per test file
- Doc comment above each test function stating intent; no comments inside test bodies

### Frontend Tests

Vitest + jsdom. Specs in `tests/js/`, run via `task test-js`, wired into `frontend-check` job in `ci.yml`. JS served as classic `<script src>` bundles — **no file in `src/assets/js` contains `export`** — suite can't import shipped code directly. `tests/js/helpers/loadScript.js` bridges: reads file, appends capture expression, injects as `<script>`, returns requested identifiers.

```js
const { exports: { proviant }, realm } = loadClassicScript("proviant.js", ["proviant"], {
  globals: { fetch: fetchStub, navigator: { onLine: true } },
  prelude: "localStorage.clear(); document.body.innerHTML = '';",
});
```

**Four constraints — verified empirically, not assumed:**

1. **Return `realm`, not `window`.** vitest's jsdom exposes DOM on test's global object, but injected script runs in jsdom's *own* realm whose `window` differs. Writes to `window` from script invisible to test. `document` only shared reference — carrier. `realm` = realm's real `window`, only handle on its `localStorage`, `fetch`, `document.cookie`.
2. **Stub globals *before* loading, via `globals`.** `proviant.js` captures `window.fetch` at load to build CSRF interceptor. Stub after wraps different function than one under test. `globals` uses `Object.defineProperty` — jsdom exposes `navigator` as getter-only accessor.
3. **Reset realm state per test.** jsdom realm lives whole file — `localStorage` + `document.body` carry over. Without `prelude` reset, one test's queue replays into next, stale badge stays element `getElementById` resolves. `localStorage.clear()` before source — source reads at load.
4. **Delegation tests need own file.** Every `theme.js` load adds document click listener calling `window.proviantTheme.cycle()` — N loads = N cycles per click, assertion would depend on test order. Hence `theme.click.test.js` alone.

`vitest.config.mjs` pins `TZ=UTC`: `formatDate` + `colorExpiry` build dates from local time, assertions drift laptop↔CI otherwise.

**No coverage task, deliberately.** Injected scripts run in jsdom's own V8 context, coverage inspector never sees — every file 0%. Provider works; normally imported module measures fine; loader defeats it. Don't add back until harness attributes script to filename: permanently-zero metric worse than none.

**Not covered yet** (IIFEs, internals unreachable without production-code seam): `homeStats.js`, `wasteAnalytics.js`, `shoppingList.js`. Also `sw.js` — fetch routing inline in `self.addEventListener('fetch')` handler, needs `caches`/`Response` stubs. Highest-risk untested code — AGENTS.md documents routing policy as deliberate. Issue #394.

### Frontend/Assets
- SCSS in `src/templates/scss/`
- Compiled CSS → `src/assets/css/`
- **`task css` after any `.scss` change** — compiled CSS served; SCSS edits without recompile no effect
- JS in `src/assets/js/`
- **Bootstrap 5** for styling/layout — Context7: `/websites/getbootstrap`
- **Bootstrap Icons** — Context7: `/twbs/icons`
- **No inline `style="..."` in templates** — CSS classes in `src/templates/scss/main.scss`. Inline OK only truly dynamic values (e.g., `width` from template var). When in doubt, class.

#### Bootstrap & Bootstrap Icons — Documentation Workflow
Consult official docs via Context7 before any HTML/CSS/template work. No memorized patterns.

| Library | Context7 ID | Use for |
|---------|-------------|---------|
| Bootstrap 5 | `/websites/getbootstrap` | Components, grid, utilities, JS plugins |
| Bootstrap Icons | `/twbs/icons` | Icon names, SVG usage, font usage |

**Query docs (mandatory):** any UI component (navbar, modal, offcanvas, accordion, toast, etc.); grid/flexbox/spacing utilities; `data-bs-*` options for JS plugins; icon name lookup before template use; correct class names for color/sizing/state variants.

**Workflow:**
1. `query-docs` w/ relevant Context7 ID + specific question.
2. Implement documented pattern — no guessing class names/attributes.
3. After SCSS change: `task css` + bump `CACHE_NAME` in `sw.js`.

#### Service Worker Caching
- Service worker (`src/assets/js/sw.js`)
- **CSS + non-JS assets** under `/assets/` cache-first — bump `CACHE_NAME` after every `task css` or fonts/icons/static change
- **JS files** network-first — always fresh, no `CACHE_NAME` bump needed
- After bump: `task css` (or `task js`) to deploy new version alongside asset change

#### Frontend JavaScript Event Handling
- **Event delegation always**: `document.addEventListener("click", ...)` + `event.target.closest()`
- **Never capture DOM refs at script load** (e.g., `const btn = document.getElementById("btnX"); btn.onclick = ...`) — stale after navigation/DOM updates
- Query DOM inside handler, not module scope
- Works regardless of browser caching or SPA-like navigation
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
- godoc per `### Comments` policy
- Swagger annotations API endpoints
- `task doc` regenerate docs before commits
- Swagger format: `@Summary`, `@Description`, `@Tags`, `@Router`
- **All API + web handlers in router MUST have Swagger annotations** — add to `src/router/router.go`, immediately add Swagger comments above declaration.

### Performance Considerations
- `strings.Builder` over concatenation in loops
- `strings.SplitSeq` (Go 1.18+)
- Pre-calc `strings.Builder` buffer sizes when known
- Avoid `fmt.Sprintf` for simple string building in hot paths

## Repository Structure
- `src/api/` — API endpoints + handlers
- `src/assets/` — static assets (CSS, JS, fonts, icons)
- `src/controllers/` — business logic
- `src/errors/` — custom errors
- `src/models/` — data models (database, API, authentication, configuration)
- `src/router/` — Gin router + middleware
- `src/templates/` — HTML templates w/ embedded assets
- `src/web/` — frontend page handlers
- `docs/` — generated docs

## Dashboard / Portal

Home page (`/web/`) fully client-side. Go handler (`web/frontend.go → Root`) provides only `HasHousehold` + `Household` — no server-side tile data.

**API endpoint:** `GET /api/v1/products/stats` (JWT) — returns `apiModel.ProductStatsResponse` in `src/models/api/stats.go`. Fields:
- `wasteCount`, `wastePercent`, `totalActive` — active/expired counts
- `totalArchived`, `uniqueArchived` — archived counts
- `lastInsertedProduct` — most recently added name
- `expiringSoon` — `[]StatsExpiringProduct` (name + date), expiring within 7 days (today inclusive), ascending
- `categories` — `map[string]int` category breakdown, active products
- `expiryTrend` — `[]StatsMonthlyCount`, next 12 calendar months

**Frontend:** `src/assets/js/homeStats.js` fetches stats on `DOMContentLoaded`, then:
1. Prepends metric tiles (`renderTile` / `renderListTile`) into `#dashboard` before `.chart-col` sentinels
2. Renders three Chart.js charts into canvases in `home.tmpl`

**Chart.js:** `src/assets/js/chart.umd.min.js` (from `node_modules` via `task js`). `<script>` in `home.tmpl` before `homeStats.js`.

**Removed:** `GetUserHomeTiles` DB method + `webparts` import from `databasecontroller.go` — tiles now entirely stats API.

## Conventional Commits

[Conventional Commits](https://www.conventionalcommits.org/) for automated changelog via git-cliff.

- **Types**: feat, fix, docs, chore, refactor, test, ci, build, style, perf, deprecat, remove, delete, security, implement, add, hotfix, bug
- **Format**: `<type>[(<scope>)]!: <description>` — e.g., `feat: add barcode scanner`, `fix(api): resolve timeout`
- **Breaking**: `!` after type/scope — e.g., `feat!: drop support for SQLite`
- **Body**: optional, blank line after subject

## Release Automation

- **CHANGELOG**: `v*` tag push → `.github/workflows/release.yml`. `git-cliff` (via `taiki-e/install-action@git-cliff`) + `cliff.toml` → CHANGELOG from conventional commits, commits back, attaches as release asset.
- **Auth**: built-in `GITHUB_TOKEN` (`permissions: contents: write`). `RELEASE_TOKEN` secret obsolete. Push-back to `main` uses `x-access-token` URL form (`https://x-access-token:${{ secrets.GITHUB_TOKEN }}@github.com/...`). Note: branch protection w/ PR reviews blocks push-back — exempt release bot or allow direct pushes for releases.
- **Release creation**: `softprops/action-gh-release@v2` → GitHub Release, uploads `CHANGELOG.md`.
- **Tag format**: `vX.Y.Z` (e.g., `v0.4.0`). `cliff.toml` pattern `v?[0-9].*` — both `v`-prefixed + legacy unprefixed.

## Important Notes
- **NEVER commit without explicit user confirmation.** Propose commit message, wait for approval. Every `git commit` + `git push`, regardless of task size or prior instructions.
- **Context7 MCP always** for codegen, setup/config, library/API docs — auto-call `resolve-library-id` + `query-docs`, no waiting.
- `task check` before commits — golangci-lint must pass
- **DB migrations**: new/modified model → add to `AutoMigrate` in `src/proviant.go` (~line 188). Omit = "no such table" at runtime. Example:
  ```go
  migrationError := dbHandle.AutoMigrate(
      &dbModel.Household{},
      &authentication.User{},
      &dbModel.Product{},
      &dbModel.HouseholdApplication{},
      &dbModel.YourNewModel{},  // ← always add new models here
  )
  ```
- **OpenFoodFacts caching**: barcode lookups via backend proxy `GET /api/v1/products/openfoodfacts/:barcode` (JWT). `openfoodfacts.cacheEnabled: true` → responses in `open_food_facts_caches` table (`src/models/database/openfoodfacts_cache.go`), served from there. Frontend (`src/assets/js/productsCreate.js`) calls `proviant.getOpenFoodFactsData()` from `proviant.js` — **never** reintroduce direct browser calls to `world.openfoodfacts.org`. Cache ops in `src/controllers/database/databasecontroller.go` (`GetOpenFoodFactsCacheByBarcode`, `CreateOpenFoodFactsCache`).
- **Config values**: new config blocks (e.g., `mailDigest`, `ntfy`, `telegram`) under `notification:`. Always all three: `config.yaml`, `config.yaml.sqlite.tmpl`, `config.yaml.mariadb.tmpl`.
- **String literals**: new constants (route labels, config keys, section identifiers) centralized in `src/util/util.go` under appropriate group comment.

## UI/UX Standards

### Color & Contrast
- CSS custom properties (Bootstrap vars) for theming — no raw hex in templates
- WCAG AA 4.5:1 min contrast, normal text
- Primary color primary CTAs only; secondary/neutral for non-essential

### Layout & Spacing
- 4pt/8pt grid via Bootstrap utilities (e.g., `p-2`, `m-3`, `gap-2`)
- Touch targets min 44×44px, buttons/links/form controls
- **No inline `style="..."`** for layout; reusable utility/component classes

### Typography
- Max five font sizes
- Hierarchy via weight + size, not color alone

### User Guidance
- Async >500ms: loading indicator + disable trigger button until done
- Primary actions `btn-primary`
- Inline validation on-blur; full-page feedback at submit only
- Empty states always clear CTA (e.g., "Add your first product")
- Explicit confirmation before irreversible actions (e.g., deletion dialogs)

### Feedback
- `proviant.showFeedback()` for page-level async results
- Inline alerts field/section-level errors only; no global banners for localized issues

### Accessibility
- `lang` on `<html>` matches active UI language
- Icon-only buttons `aria-label` describing action
- All interactive keyboard-navigable (`tabindex`, `focus-visible` styles)
- Populate `aria-live` after dynamic updates for screen reader announcements

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
