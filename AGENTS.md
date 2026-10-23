# Agent Guidelines for Proviant

Agent coding guidelines for Proviant codebase.

**Keep accurate.** Cite symbols not line numbers (`AutoMigrate` in `proviant.go`, not "line 188") — they rot. Cite durable code + docs not issue numbers — `#123` closes, means nothing later; state rule. Exception: underivable exact facts — pinned digests, advisory IDs, version strings — live in script they govern, not here.

## Search order (mandatory)

1. Architecture, relationships, "what calls this", blast radius, onboarding: if `graphify-out/graph.json` exists, run `graphify query "<question>"` first (`graphify path` / `graphify explain` for specific nodes). Never grep for this.
2. Finding an implementation or a name you don't know: call `tools.semble.search` through `execute`, then read the returned file and lines. `tools.semble.find_related` finds similar code elsewhere.
3. `grep` only for exhaustive literal occurrences (rename, config key, flag) or after 1–2 failed. `glob` only for path patterns.
4. After changing code, run `graphify update .` (see the graphify section for the full rules).

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
- Group: stdlib → internal (`github.com/Isotop7/proviant/…`) → external.
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

Vitest + jsdom, specs in `tests/js/`, run via `task test-js` (wired into the `frontend-check` CI job). Shipped JS is classic `<script src>` bundles — **no file in `src/assets/js` contains `export`**, with one exception: `src/assets/js/bootstrap-globals.js` is an ES module (see *Bootstrap 6* below) and no test loads it. So `tests/js/helpers/loadScript.js` injects the file as a `<script>` and returns the requested identifiers:

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
- **Bootstrap 6** for layout/styling and **Bootstrap Icons**. **Consult Context7 before any HTML/CSS/template work** — no memorized patterns: `/websites/getbootstrap` (components, grid, utilities, `data-bs-*` JS plugin options) and `/twbs/icons` (icon names, SVG/font usage). Then implement the documented pattern.
- **No inline `style="…"` in templates** — use classes in `src/templates/scss/main.scss` (inline only for truly dynamic values, e.g. a template-computed `width`).
- **Event delegation always**: `document.addEventListener("click", …)` + `event.target.closest()`, and query the DOM inside the handler. Never capture element refs at script load — they go stale after navigation/DOM updates.
- After a `.scss`/font/icon change: `task css` **and** bump `CACHE_NAME` in `src/assets/js/sw.js`. The service worker caches CSS + non-JS `/assets/` cache-first; JS is network-first (no bump needed). A forgotten bump leaves every browser on the previous stylesheet — the symptom is "the fix is in the repo but the screen did not change".

### Bootstrap 6

Bootstrap 6 (currently `6.0.0-alpha.1`) renames whole component families and drops the `--bs-` prefix from every custom property it emits. Four things differ from v5 and bite in practice:

- **The bundle is ESM-only.** `src/assets/js/bootstrap-globals.js` is the single ES module in `src/assets/js`: it imports `bootstrap.bundle.min.js` and assigns `window.bootstrap`, so the classic scripts keep reaching for `bootstrap.Dialog`, `bootstrap.Menu`, `bootstrap.Toast`. The layouts load it with `<script type="module">`. Module scripts are deferred, so it runs before `DOMContentLoaded` — every Bootstrap call in the app happens from an event handler or a `DOMContentLoaded` listener, both of which see the global.
- **Renamed components**: Modal → `Dialog` (a native `<dialog>`, no `.modal-dialog`/`.modal-content` wrappers, `data-bs-toggle="dialog"`), Offcanvas → `Drawer` (also a native `<dialog>`), Dropdown → `Menu` (no `.dropdown` wrapper, no `.dropdown-toggle`, the list flattens to plain children), Accordion → native `<details>`/`<summary>`. The JS names follow: `bootstrap.Dialog`, `bootstrap.Menu`, and the events `shown.bs.dialog` / `hidden.bs.dialog`.
- **Unprefixed custom properties.** Bootstrap 6 emits `--fg-*`, `--bg-*`, `--border-*`; the `--bs-` prefix only exists in the prebuilt dist CSS. `src/templates/scss/main.scss` configures Bootstrap's token maps (`$theme-colors`, `$theme-bgs`, `$theme-fgs`, `$root-tokens`) from the design system's Sass variables, so both layers read one source. The five names that collide with the design system are aligned by hand — see `design-system/deviations.md` #38–#40.
- **New utility spellings**: `btn-primary` → `btn-solid theme-primary`, `text-body-secondary` → `fg-secondary`, `d-md-flex` → `md:d-flex`, `col-md-6` → `md:col-6`, `form-select` → `form-control` on the `<select>`, `form-check-input` → `check`/`radio` (switches keep a `.switch` wrapper), `needs-validation` → `data-bs-validate` on the `<form>`.
- **Utilities lost their `!important`.** v5 emitted every utility with `!important`; v6 dropped that and moved the utilities into the `utilities` cascade layer, while this app's stylesheet stays unlayered — so an app rule now beats a utility of the same specificity (`.btn`'s own `display: inline-flex` beats `.d-none`, and the button stays visible). The app relies on the v5 precedence in a few dozen places, so `src/templates/scss/_bootstrap-utilities.scss` generates the utilities with `important: true` instead of pulling in `bootstrap/scss/utilities/api`. Do not switch back to the upstream API without re-checking every display utility.
- **`.card-header` and `.card-body` are flex columns.** A card header that carries a title and a button needs an explicit `flex-row`; a card body that the app spaces with its own utilities stays a block (see `deviations.md` #42).

`task lint-selectors` (`scripts/check-js-class-selectors.py`) checks that every class the JavaScript queries still exists in a template, a stylesheet or Bootstrap itself. A stale selector does not fail loudly — `querySelector` returns null and the code path quietly stops working. Run it after renaming anything in the markup.

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

The design system lives in `design-system/` and is binding. Read
`design-system/DESIGN.md` before touching markup or styles, and record every
departure in `design-system/deviations.md`.

- **Tokens**: `design-system/_tokens.scss` is the only source. Change it there, run `task design-tokens`, never edit `design-system/tokens.css` or `tokens.json` by hand. `task design-check` fails a stale artifact, a missing token or a contrast below its target.
- **Color**: theme via the design system's CSS custom properties, no raw hex and no `oklch()` in the app's own styles. `--accent` fills, `--accent-text` writes; the status colours carry a `-text` variant for labels and the plain value for icons, dots and bars.
- **Components**: Bootstrap 6 first. `.btn`, `.badge`, `.form-control`, `.list-group` and their modifiers carry the brand; own classes only where Bootstrap has none (`.kpi`, `.toast-stack`, `.list-group-item__*`, `.bento*`).
- **Layout**: 4/8pt spacing via the spacing tokens or Bootstrap utilities; touch targets ≥44×44px; the app shell is `.app` with `.app__sidebar` and `.app__topbar`.
- **Type**: Jost for the interface, DMMono for numbers and codes; hierarchy via weight and size, not colour alone.
- **Guidance**: async >500ms → loading indicator + disable the trigger; inline validation on blur, page-level feedback at submit; empty states carry a clear CTA; confirm before irreversible actions.
- **Feedback**: `proviant.showFeedback()` shows a toast, not a modal; field and section errors stay inline. A toast's callback fires when it goes — the auto-hiding types after their timeout, an error when it is dismissed.
- **Accessibility**: `<html lang>` matches the active UI language; the page header carries the page's only `h1` — the topnav is navigation; icon-only buttons get an `aria-label`; everything keyboard-navigable (`tabindex`, `focus-visible`); populate `aria-live` after dynamic updates.

## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

When the user types `/graphify`, use the installed graphify skill or instructions before doing anything else.

Rules:
- When graphify-out/graph.json exists, run `graphify query "<question>"` before your first grep, search or source-file read for a codebase question, and put this rule in every subagent prompt that explores code. Read raw files only for lines the graph or an earlier read already located. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- Dirty graphify-out/ files are expected after hooks or incremental updates; dirty graph files are not a reason to skip graphify. Only skip graphify if the task is about stale or incorrect graph output, or the user explicitly says not to use it.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).
- A committed graphify-out/graph.json is rewritten whole on every build, so a large diff to it in a pull request is expected. Do not flag it as churn or suggest skipping the graph rebuild or its hooks.
