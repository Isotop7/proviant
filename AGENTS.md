# Agent Guidelines for Proviant

Agent coding guidelines for Proviant codebase.

**Keep accurate.** Cite symbols not line numbers (`AutoMigrate` in `proviant.go`, not "line 188") — they rot. Cite durable code + docs not issue numbers — `#123` closes, means nothing later; state rule. Exception: underivable exact facts — pinned digests, advisory IDs, version strings — live in script they govern, not here.

## Workflow

1. Locate code with search before reading; read a full file only when the located excerpt lacks context.
2. Library/API/config questions: Context7 MCP (`resolve-library-id` + `query-docs`), automatically, no asking.

## Build, Lint, and Test Commands

**Prefer `task` wrappers** over vanilla `npm`/`go`/`podman`. The Taskfile centralizes flags, paths, and side-effects (cache dirs, font/icon copy, container mounts) so the same task behaves identically across machines.

| Task | Use when… |
|------|-----------|
| `task init` | First-time setup, or after `package.json`/`go.mod` change. |
| `task css` | `.scss` under `src/templates/scss/` edited (compiles to `src/assets/css/`). |
| `task js` | New/changed frontend npm dep (bootstrap, chart.js, html5-qrcode) copied into `src/assets/js/`. |
| `task icons` / `task fonts` | `res/icons/` or font files changed (`fonts` runs as a dep of `css`). |
| `task lint` (`-css`/`-js`/`-html`) | Before frontend commits. |
| `task vet` / `task tidy` | Quick `go vet`; before Go commits (`go fmt` + `go mod tidy`). |
| `task test` / `task test-js` | Go suite `-race`; Vitest suite in `tests/js/`. |
| `task check` / `task check-changed` | Go lint via the golangci-lint container; only files changed vs `HEAD`. |
| `task doc` | After adding/changing exported Go symbols or Swagger annotations. |
| `task run` | Local dev server. |
| `task containerimage` / `task runcontainer` / `task runcontainerdebug` | Build/run containerized app. |
| `task vuln` / `task vuln-go` / `task vuln-npm` | Security audits (Go + npm). |
| `task vuln-image` | Scan container image (needs podman/docker; excluded from `task vuln`). |
| `task seed` / `task seed-reset` | Populate / wipe demo products. |

**Never call the underlying tools directly — always the task**: `npm install` / `npm test` / `npm run *` → `task init` / `test` / `css` / `js` / `lint` (`npm run test-js` → `task test-js`); `podman`/`docker build` → `task containerimage`; `golangci-lint run` → `task check`; `go run` for the dev server → `task run`. Single test: `cd src && go test -race -vet=off ./path/to/package [-run TestName]`.

## Dependency Auditing

`npm audit` gates **production deps only** (`--omit=dev`); dev findings are reported but never fail. The policy lives once in `scripts/npm-audit.sh` — never inline `npm audit` into a workflow or the Taskfile. That script's header documents the standing dev advisory (`braces`), why it is unreachable at runtime, and the re-check trigger; never reintroduce `npm audit fix --force` or a `braces` `overrides` entry.

## Container Image Scanning

`trivy` scans the prod image (base `alpine` + tesseract OS packages — invisible to `govulncheck`/`npm audit`). Gate flags live in `scripts/image-scan.sh`: `--scanners vuln --severity HIGH,CRITICAL --ignore-unfixed --exit-code 1`; unfixed findings never fail the build (no maintainer action — fix by bumping the base image). Never inline `trivy` flags into a workflow or the Taskfile; edit the script. Run local via `task vuln-image`. CI wiring, digest promotion, and cache-key rationale are documented in the workflow comments (`.github/workflows/ci.yml`, `release.yml`, `weekly-vuln-scan.yml`).

Rules that must not regress:

- **Digest pin, never a version tag** (`$TRIVY_IMAGE`) for the container path — mutable tags were poisoned in the March 2026 trivy supply-chain compromise (`GHSA-69fq-xp46-6x23`). Bump the pin in **both** `scripts/image-scan.sh` and the Taskfile `TRIVY_IMAGE`, then verify `task vuln-image`; registry tags are unprefixed (`0.75.0`, not `v0.75.0`).
- **Promotion stays digest-based** (`docker buildx imagetools create`). Never `pull` + `tag` + `push` — that copy carries neither attestations nor the link to the scanned report.
- **Fork PRs run no image scan**; the gate is enforced by branch protection on `main`/`develop`, not PR CI.

## Code Style Guidelines

### Imports
- Group: stdlib → internal (`codeberg.org/…`) → external.
- Side-effect imports use `_` and a one-line why.

### Formatting
- `task tidy` before commits (`go fmt` + `go mod tidy`); tabs; ~120 chars max (unenforced).

### Comments

Comments are a last resort — reach for **a better name** → **a helper** → **the test name** → **the issue/PR description** first. Only then write one line, and never repeat the identifier.

Allowed (complete list): godoc on exported symbols; Swagger on handlers; one intent comment above a test; one line on a side-effect import; a constraint a reader cannot infer (wire-format contract, silent-failure coupling, values that must change together).

Everything else is discouraged — restating what code does, narrating steps, explaining a workaround. A paragraph means the code is wrong: split it or find the name that deletes it.

```go
// WRONG: 4-line rationale above obvious code → logProbeOutcome(ctx, err)
// CORRECT: the name carries it, no comment → logClientAbandonedProbe(ctx, err)
```

### Naming
- Exported symbols PascalCase (`GetProduct`); unexported camelCase (`setupConfig`); packages lowercase single words (`database`).
- Constants PascalCase (`PreferredTimeFormat`); interfaces PascalCase functional (`Authorizator`).
- Error vars carry an `Err` prefix (`ErrInvalidUserID`) and live in `src/errors/errors.go` — custom errors only, no generic strings.
- Variables fully descriptive, never abbreviated (`storageLocationRepo`, `householdID` — not `slRepo`, `hID`).

### Types and Structs
- Embed `gorm.Model` (ID, CreatedAt, UpdatedAt, DeletedAt); GORM tags `gorm:"index, not null"`; JSON `json:"fieldName"`; `json:"-"` hides sensitive fields.
- Barcode is EAN-13 (13 alphanumeric digits).

### Error Handling
- Return errors (no `panic` in normal flow).
- **Early returns over nested ifs** — check and return first, keep main logic at base indentation.

```go
if !ok { return }
if tokenErr != nil { return }
// main logic here
```

### Database Operations
- GORM for all queries. Multi-step work in a transaction: `tx := db.Begin()` → `tx.Commit()` / `tx.Rollback()`.
- Soft delete via `DeletedAt`; `.Unscoped()` for a hard delete. Not-found: `if err.Error != gorm.ErrRecordNotFound`.

### API Handlers (Gin)
- `logger, _ := ctx.MustGet("logger").(*zerolog.Logger)`; `dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)`; `claims := jwt.ExtractClaims(ctx)`.
- Use proper HTTP codes (`ctx.JSON(http.StatusBadRequest, api.APIResponse{...})`); handle the `!ok` return from `ctx.MustGet`.

### Logging
- zerolog logger from the Gin context — never create a new one. Chain a level and terminate with `.Msg()` / `.Msgf()` (`logger.Warn().Msg(err.Error())`).

| Level | When to use | Examples |
|-------|-------------|----------|
| **Trace** | SQL tracing, extreme debug detail | GORM adapter SQL tracing |
| **Debug** | Dev-time info | migration steps, cache hit/miss, event processing |
| **Info** | Normal production-worthy events | startup, user created, config loaded, cleanup counts, notification sent |
| **Warn** | Recoverable, expected, user-caused — no immediate action | invalid input (4xx), requested-not-found, background task failure, cache errors, file close errors |
| **Error** | Unexpected server-side failure needing attention | DB/external-API failures, missing required context, 5xx, internal logic errors |
| **Fatal** | Immediate startup termination | cannot bind port, cannot set trusted proxies |

### Testing

Helpers in `src/testutil/`: `SetupTestDB(t)` (in-memory SQLite, all migrations), `SetupGinContext(db)` (Gin context + logger + db), `MockJWTClaims(ctx, userID)` / `MockJWTClaimsWithKey(ctx, userID, key)`, `CreateTestUser(db, householdID)`, `CreateTestHousehold(db, adminID)`, `CreateTestProduct(db, householdID)`, `CreateTestStorageLocation(db, householdID)`.

- `t.Run()` all subtests; cover success **and** error paths.
- Plain Go assertions (`if got != want { t.Errorf(...) }`) preferred; testify/assert also fine.
- DB setup lives in the helpers, not per test file.
- One doc comment above each test stating intent; no comments inside test bodies.

### Frontend Tests

Vitest + jsdom, specs in `tests/js/`, run via `task test-js` (wired into the `frontend-check` CI job). Shipped JS is classic `<script src>` bundles — **no file in `src/assets/js` contains `export`** — so `tests/js/helpers/loadScript.js` injects the file as a `<script>` and returns the requested identifiers:

```js
const { exports: { proviant }, realm } = loadClassicScript("proviant.js", ["proviant"], {
  globals: { fetch: fetchStub, navigator: { onLine: true } },
  prelude: "localStorage.clear(); document.body.innerHTML = '';",
});
```

Four constraints, each verified empirically (not assumed):

- Return `realm`, not the test's `window` — the injected script runs in jsdom's own realm and only `document` is shared.
- Stub globals **before** load, via `globals` (source captures globals at load time).
- Reset realm state per test — the realm (and its `localStorage`/`document.body`) persists for the whole file.
- Delegation tests need their own file — each load adds a click listener, so N loads fire N times per click.

`vitest.config.mjs` pins `TZ=UTC` (date assertions drift laptop↔CI otherwise). **No coverage task, by design** — injected scripts run outside the coverage inspector, so every file reports 0%.

### Frontend / Assets

- SCSS in `src/templates/scss/` compiles to `src/assets/css/`; **run `task css` after any `.scss` change** or the served CSS is stale. JS lives in `src/assets/js/`.
- **Bootstrap 5** for layout/styling and **Bootstrap Icons**. **Consult Context7 before any HTML/CSS/template work** — no memorized patterns: `/websites/getbootstrap` (components, grid, utilities, `data-bs-*` JS plugin options) and `/twbs/icons` (icon names, SVG/font usage). Then implement the documented pattern.
- **No inline `style="…"` in templates** — use classes in `src/templates/scss/main.scss` (inline only for truly dynamic values, e.g. a template-computed `width`).
- **Event delegation always**: `document.addEventListener("click", …)` + `event.target.closest()`, and query the DOM inside the handler. Never capture element refs at script load — they go stale after navigation/DOM updates.
- After a `.scss`/font/icon change: `task css` **and** bump `CACHE_NAME` in `src/assets/js/sw.js`. The service worker caches CSS + non-JS `/assets/` cache-first; JS is network-first (no bump needed).

### Documentation
- godoc per the Comments policy; Swagger annotations (`@Summary`, `@Description`, `@Tags`, `@Router`) on **every** API and web handler — add the comment in `src/router/router.go` the moment you add the handler.
- Run `task doc` to regenerate docs before committing (never hand-edit `src/docs/`).

### Performance
- `strings.Builder` (pre-size the buffer when known) over loop concatenation; `strings.SplitSeq`; avoid `fmt.Sprintf` for simple string building in hot paths.

## Repository Structure
`src/api/` handlers · `src/controllers/` business logic · `src/errors/` custom errors · `src/models/` (database, api, authentication, configuration) · `src/router/` Gin router + middleware · `src/templates/` HTML + SCSS · `src/assets/` compiled CSS/JS/icons/fonts · `src/web/` page handlers · `src/docs/` generated godoc + Swagger (written by `task doc`, never hand-edited).

## Dashboard / Portal

Home (`/web/`) is fully client-side: `web/frontend.go → Root` passes only `HasHousehold` + `Household` — no server-side tile data. Tiles come from `GET /api/v1/products/stats` (JWT) → `apiModel.ProductStatsResponse` (`src/models/api/stats.go`): waste/active/archived counts, `lastInsertedProduct`, `expiringSoon` (next 7 days), `categories`, `expiryTrend` (next 12 months). `src/assets/js/homeStats.js` renders the tiles and three Chart.js charts (`home.tmpl`) on `DOMContentLoaded`; Chart.js is `src/assets/js/chart.umd.min.js` (via `task js`).

## Internationalization (i18n)

- Catalogs live in `src/i18n/catalogs/{en,de}/<namespace>.json` (flat key→string). Go: `i18n.T(lang, "ns.key", args...)` (placeholders: `%s`/`%d`/`%v` in order; `i18n.Tn` for plurals, key + `_one`/`_other`). Templates: `{{ t "ns.key" }}` / `{{ tn "ns.key" .N }}`; JS: `proviant.t("ns.key")` / `proviant.tn("ns.key", n)`. The server injects `window.proviantI18n` per page in `base.tmpl`.
- `<html lang="{{ .Lang }}">` (dynamic) — **closes #346 as part of i18n; never fix separately.** Language chain: user pref (`User.Language`) → household pref (`Household.Language`) → `Accept-Language` → `en`. Unknown/empty means automatic.
- Add the `Language` field handling to API PATCH requests via `*string` (nil = unchanged).
- Every template's user-visible string becomes a catalog key. English catalog value must be byte-identical to the replaced string; `common.*`/`nav.*`/`a11y.*`/`status.*`/`error.*` shared keys are reused, not duplicated. Markup stays in code; catalog values are plain text.
- Template output (`t`) is HTML-escaped — no HTML in catalog values; put `<br>` in the template.
- Tests: new catalogs need `task i18n` (parity + referenced-key checks) green; JS tests get the en catalog injected by `tests/js/helpers/loadScript.js`. UI strings rendered server-side are testable with `lang=en` default.
- Notification providers take a `lang` arg; resolve recipient language at dispatch via `i18n.ResolveLanguage`.
- The `CACHE_NAME` in `sw.js`: bump only on CSS/font/icon changes, not JS/strings.

## Conventional Commits

[Conventional Commits](https://www.conventionalcommits.org/) drive the git-cliff changelog. Format `<type>[(<scope>)]!: <description>` (`!` = breaking) — e.g. `feat: add barcode scanner`, `fix(api): resolve timeout`. Types: feat, fix, docs, chore, refactor, test, ci, build, style, perf, deprecat, remove, delete, security, implement, add, hotfix, bug. Body optional, blank line after subject.

## Release Automation
`v*` tag → `.github/workflows/release.yml` runs git-cliff (`cliff.toml`) to build `CHANGELOG.md`, commits it back to `main`, and attaches it to a GitHub Release (`softprops/action-gh-release@v2`), using the built-in `GITHUB_TOKEN` (`contents: write`; `RELEASE_TOKEN` is obsolete). Tags are `vX.Y.Z`; `cliff.toml` matches `v?[0-9].*`. Branch protection with required PR reviews blocks the `main` push-back — exempt the release bot or allow direct pushes for releases.

## Important Notes

- **NEVER commit or push without explicit user confirmation.** Propose the message and wait for approval — every `git commit` and `git push`, regardless of task size or prior instructions.
- Run `task check` (golangci-lint must pass) before committing.
- **DB migrations**: add every new/modified model to the `AutoMigrate` call in `src/proviant.go` — omitting it means a runtime "no such table".
- **Config values**: a new block under `notification:` (e.g. `mailDigest`, `ntfy`, `telegram`) must be added to all three: `config.yaml`, `config.yaml.sqlite.tmpl`, `config.yaml.mariadb.tmpl`.
- **String literals**: centralize new route labels / config keys / section identifiers in `src/util/util.go` under the appropriate group comment.
- **OpenFoodFacts**: barcode lookups go through the backend proxy `GET /api/v1/products/openfoodfacts/:barcode` (JWT), cached in `open_food_facts_caches` when `openfoodfacts.cacheEnabled: true`. The frontend calls `proviant.getOpenFoodFactsData()` — **never** call `world.openfoodfacts.org` from the browser. Cache ops: `src/controllers/database/product_repository.go` (`GetOpenFoodFactsCacheByBarcode`, `CreateOpenFoodFactsCache`).

## UI/UX Standards

- **Color**: theme via CSS custom properties (Bootstrap vars), no raw hex; primary color for primary CTAs only; WCAG AA ≥4.5:1 for normal text.
- **Layout**: 4/8pt spacing via Bootstrap utilities (`p-2`, `m-3`, `gap-2`); touch targets ≥44×44px.
- **Type**: ≤5 font sizes; hierarchy via weight + size, not color alone.
- **Guidance**: async >500ms → loading indicator + disable the trigger; inline validation on blur, page-level feedback at submit; empty states carry a clear CTA; confirm before irreversible actions.
- **Feedback**: `proviant.showFeedback()` for page-level async results; inline alerts for field/section errors — no global banner for a local problem.
- **Accessibility**: `<html lang>` matches the active UI language; icon-only buttons get an `aria-label`; everything keyboard-navigable (`tabindex`, `focus-visible`); populate `aria-live` after dynamic updates.
