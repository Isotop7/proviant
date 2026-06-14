# Plan: Apply `design-system/` Tokens & Common Components to `src/templates/scss/`

> Source of truth: `design-system/DESIGN.md`, `design-system/tokens.css`, `design-system/components.manifest.json`, `design-system/screens/*.html`.
> Target: `src/templates/scss/main.scss` and `src/templates/scss/login.scss`.
> Tokens stay inlined in `main.scss` `:root` (per user decision). `design-system/` remains a reference.
> Deviations from the spec are tracked in `design-system/deviations.md` (new file).

## Scope

**In scope (this pass):**
1. Rewrite `:root` token block in `main.scss` to match `design-system/tokens.css` exactly.
2. Drop `--brand-slate`, `--brand-slate-subtle`, `--status-nodate-*`, `--color-info: blue` (replace `--color-info` with design-system's `--info: #5A7A8A`).
3. Rename internal token aliases to design-system names (`--accent-fg` → `--accent-on`, `--accent-subtle` → `--accent-soft`, `--bg-subtle` → `--surface-warm`, `--fg-3` → `--meta`, `--border-subtle` removed, `--shadow-*` → `--elev-*`).
4. Full Bootstrap button migration: replace `.btn-proviant-primary/secondary/danger`, `.btn-ghost` in **all `.tmpl` files** + any JS that toggles them, with `.btn-primary` / `.btn-outline-secondary` / `.btn-link` / `.btn-danger`. Delete the custom CSS blocks.
5. Adopt design-system login brand panel: `BG --surface-warm`, dark text, no decorative circles. Rewrite `.login-brand`, `.brand-headline`, etc. in `login.scss`. Keep the layout (50 % split on desktop, mobile logo only).
6. Add common component classes per `components.manifest.json`:
   - `.eyebrow`
   - `.stack-2` / `.stack-3` / `.stack-4` / `.stack-6`
   - `.badge--fresh|soon|critical|expired|neutral` (with `color-mix(18%)` subtle BG)
   - `.list-group-item__leading|body|title|meta|trailing`
   - `.field--code`
7. Update Bootstrap variable overrides (`$primary`, `$secondary`, `$info`, `$body-bg`, `$body-color`, `$border-color`, `$line-height-base`) to mirror new token values, so vanilla `.btn-primary` / `.form-control` / `.card` / `.table` inherit the palette.
8. Add `--bs-*` mirrors in `:root` (`--bs-primary`, `--bs-body-bg`, `--bs-body-color`, `--bs-border-color`, `--bs-border-radius`, etc.) per `DESIGN.md` § 5.
9. Bump `CACHE_NAME` in `src/assets/js/sw.js` (CSS is cache-first).
10. Create `design-system/deviations.md` listing: skipped components (sidebar, KPI tiles, toast--*, table-empty, .kpi--focus, full .auth grid, .skeleton) and any token remap decisions.
11. Run `task css` to recompile, then `task lint-css` and `task lint-html`.

**Out of scope (documented in deviations.md):**
- `.app__sidebar` / `.app__topbar` / `.app__main` grid (current uses bottom-nav only; no sidebar layout today)
- `.kpi`, `.kpi__*`, `.kpi--focus` (no KPI tiles in current code)
- `.toast--info|success|warn|danger` modifiers (current uses ad-hoc toasts)
- `.table-empty`
- `.skeleton` (current has `.skeleton-block`)
- `.modal--demo`, `.toast-stack`
- `.auth`, `.auth__brand`, `.auth__form` grid (login uses bespoke `.login-root` / `.login-brand` / `.login-form-panel` — these get refactored to use the new tokens but the class names stay bespoke)
- `.page-nav`

## File-by-file changes

### `src/templates/scss/main.scss`

1. **Lines 1–85 (Bootstrap variable overrides):** update to new palette.
   - `$primary: #5A8A7A` (was `#2d7858`)
   - `$secondary: #6B6B6B` (was `#4a6fa0` — slate blue removed; use `--muted`)
   - `$info: #5A7A8A` (was `#7bc67e` — leaf green removed)
   - `$light: #F1ECE0` (was `#f0ece6` — match new `--surface-warm`)
   - `$dark: #2A2A2A` (was `#221e18`)
   - `$body-bg: #FAF7F1` (was `#f7f4ef`)
   - `$body-color: #2A2A2A` (was `#221e18`)
   - `$border-color: #E5DFD3` (was `#dad6d0`)
   - `$success: #5A8A7A`, `$warning: #C2A24A`, `$danger: #A24A4A` (recalibrated to design-system values)
   - `$link-color: #5A8A7A` (was slate — now accent)
   - `$line-height-base: 1.55` (was 1.6)
   - `$card-cap-bg: #F1ECE0`, `$card-bg: #FFFFFF`

2. **Lines 90–274 (`:root` block):** full rewrite to match `design-system/tokens.css`:
   - Schema slots: `--bg #FAF7F1`, `--surface #FFFFFF`, `--fg #2A2A2A`, `--muted #6B6B6B`, `--border #E5DFD3`, `--accent #5A8A7A`.
   - Extended slots: `--surface-warm #F1ECE0`, `--fg-2 #4A4A4A`, `--meta #8A8A8A`, `--border-strong #D5CDB8`, `--accent-soft #DDE7E0`, `--accent-on #FFFFFF`, `--shadow-warm rgba(80,60,30,0.08)`.
   - Status quadrupel: `--status-fresh #5A8A7A`, `--status-soon #C2A24A`, `--status-critical #C26A3F`, `--status-expired #A24A4A`.
   - Semantic aliases: `--success`, `--warn`, `--danger` aliased to status; `--info #5A7A8A`.
   - Typography: `--font-display/body: "VendSans", system-ui, …`, `--font-mono: "DMMono", ui-monospace, …`. Type scale `--text-display 32px` … `--text-eyebrow 11px`. Leading, tracking, weight tokens.
   - Spacing: `--space-1` … `--space-16` (4-px base, units kept in `px` to match existing pattern; spec uses `px` too).
   - Radius: `--radius-sm/md/lg/pill`.
   - Elevation: `--elev-flat`, `--elev-raised`, `--elev-overlay`, `--elev-inset`, `--elev-focus`, `--focus-ring 0 0 0 3px var(--accent-soft)`.
   - Motion: `--motion-fast 120ms`, `--motion-base 180ms`, `--motion-slow 240ms`, `--ease-out cubic-bezier(0.23, 1, 0.32, 1)`.
   - Layout: `--container-max 1120px`, `--sidebar-width 240px` (unused this pass, kept for future), `--topbar-height 56px`, `--field-height 40px` / `-hero 48px` / `-compact 32px`.
   - **Drop:** `--accent-hover`, `--accent-active`, `--accent-subtle` (use `--accent-soft` + `color-mix`), `--brand-slate*`, `--status-nodate*`, `--color-info: blue`, `--border-subtle`, `--surface-raised`, `--accent-fg`, `--fg-3`, `--shadow-xs/sm/md/lg/xl`, `--btn-primary-bg/…` aliases.
   - **Add Bootstrap mirrors:** `--bs-primary: var(--accent)`, `--bs-body-bg: var(--bg)`, `--bs-body-color: var(--fg)`, `--bs-border-color: var(--border)`, `--bs-border-radius: var(--radius-md)`, `--bs-border-radius-sm: var(--radius-sm)`, `--bs-border-radius-lg: var(--radius-lg)`, `--bs-success: var(--success)`, `--bs-warning: var(--warn)`, `--bs-danger: var(--danger)`, `--bs-info: var(--info)`.
   - **Add prefers-reduced-motion** block from `tokens.css` line 141.

3. **Lines 293–380 (`.btn-proviant-*`, `.btn-ghost`):** delete entirely. Replace with thin Bootstrap-overrides block that polishes `.btn`, `.btn-primary`, `.btn-outline-secondary`, `.btn-link`, `.btn-danger` per `DESIGN.md` § 5:
   - `text-transform: uppercase`, `letter-spacing: var(--tracking-button)`, `font-weight: var(--weight-semibold)`, `border-radius: var(--radius-pill)`, padding `var(--space-2) var(--space-4)`, `min-height: var(--field-height)`.
   - `.btn-primary { --bs-btn-bg: var(--accent); --bs-btn-border-color: var(--accent); --bs-btn-color: var(--accent-on); --bs-btn-hover-bg: var(--accent); --bs-btn-hover-border-color: var(--accent); --bs-btn-active-bg: var(--accent); }` with hover via `color-mix(in oklab, var(--accent), black 6%)`.
   - `.btn-outline-secondary { --bs-btn-color: var(--fg); --bs-btn-border-color: var(--border); --bs-btn-bg: var(--surface); }`.
   - `.btn-link { --bs-btn-color: var(--fg); text-decoration: none; --bs-btn-hover-color: var(--accent); }` with hover BG `var(--accent-soft)`.
   - `.btn-danger { --bs-btn-bg: var(--status-expired); --bs-btn-border-color: var(--status-expired); }`.
   - `:focus-visible { box-shadow: var(--focus-ring); outline: none; }`.
   - `:disabled` for all four variants → BG `var(--border)`, FG `var(--muted)`, no pointer.
   - `.btn[aria-busy="true"]` → keep padding, hide text via `color: transparent` and show `.spinner-border-sm` (Bootstrap).

4. **Lines 382–415 (form controls):** keep, but switch focus-shadow value to `var(--focus-ring)`. Replace `oklch(... / .15)` literals with `var(--accent-soft)` where possible, or keep literals if `color-mix` is needed.

5. **Lines 417–429 (`.card`):** change `box-shadow: var(--shadow-sm)` to `box-shadow: var(--elev-flat)`, hover → `var(--elev-raised)`. Min-height 150 px stays (project pattern, not in design-system — note in deviations).

6. **Add new component classes** (after the existing `.card` block):
   ```scss
   .eyebrow {
     font: var(--weight-semibold) var(--text-eyebrow)/var(--leading-tight) var(--font-body);
     letter-spacing: var(--tracking-eyebrow);
     text-transform: uppercase;
     color: var(--muted);
   }
   .stack-2 { gap: var(--space-2); display: flex; flex-direction: column; }
   .stack-3 { gap: var(--space-3); display: flex; flex-direction: column; }
   .stack-4 { gap: var(--space-4); display: flex; flex-direction: column; }
   .stack-6 { gap: var(--space-6); display: flex; flex-direction: column; }

   .badge {
     --bs-badge-padding-x: var(--space-2);
     --bs-badge-padding-y: var(--space-1);
     --bs-badge-font-size: var(--text-xs);
     --bs-badge-font-weight: var(--weight-semibold);
     --bs-badge-border-radius: var(--radius-sm);
     text-transform: uppercase;
     letter-spacing: var(--tracking-eyebrow);
   }
   .badge--fresh    { color: var(--status-fresh);    background: color-mix(in oklab, var(--status-fresh)    18%, var(--surface)); }
   .badge--soon     { color: var(--status-soon);     background: color-mix(in oklab, var(--status-soon)     18%, var(--surface)); }
   .badge--critical { color: var(--status-critical); background: color-mix(in oklab, var(--status-critical) 18%, var(--surface)); }
   .badge--expired  { color: var(--status-expired);  background: color-mix(in oklab, var(--status-expired)  18%, var(--surface)); }
   .badge--neutral  { color: var(--muted);            background: color-mix(in oklab, var(--muted)            18%, var(--surface)); }

   .list-group-item {
     min-height: 56px;
     padding: var(--space-3) var(--space-4);
     background: var(--surface);
     color: var(--fg);
     border-color: var(--border);
   }
   .list-group-item.active {
     color: var(--accent);
     background: var(--accent-soft);
     border-color: var(--accent-soft);
   }
   .list-group-item__leading  { display: flex; align-items: center; gap: var(--space-3); }
   .list-group-item__leading > .avatar,
   .list-group-item__leading > img { width: 40px; height: 40px; border-radius: var(--radius-md); }
   .list-group-item__body     { display: flex; flex-direction: column; gap: 2px; min-width: 0; flex: 1; }
   .list-group-item__title    { font-size: var(--text-base); font-weight: var(--weight-semibold); color: var(--fg); }
   .list-group-item__meta     { font-size: var(--text-sm);   color: var(--muted); }
   .list-group-item__trailing { display: flex; align-items: center; gap: var(--space-2); }

   .field--code {
     display: flex;
     gap: var(--space-2);
   }
   .field--code input {
     width: 48px;
     height: 56px;
     font: var(--weight-regular) var(--text-h3)/1 var(--font-mono);
     text-align: center;
     color: var(--fg);
     background: var(--surface);
     border: 1px solid var(--border);
     border-radius: var(--radius-md);
   }
   .field--code input:focus-visible {
     border-color: var(--accent);
     box-shadow: var(--focus-ring);
     outline: none;
   }
   ```

7. **Lines 1215 (`.skeleton-block`):** leave in place; document in deviations.md (design-system has `.skeleton`, current has `.skeleton-block` — small naming diff, no behaviour gap).

8. **Lines 1253–1300 (`.bottom-nav`, `.bottom-nav-item`):** unchanged structurally. Touch only if any token literal needs updating (the search showed no `oklch` literals in that block, only var refs — should be fine).

9. **Lines 1667, 1674 (`.page-header-title/subtitle`):** leave; project-internal.

10. **Lines 2615–3017 and elsewhere (`--accent-subtle` usages):** replace `var(--accent-subtle)` with `var(--accent-soft)` globally. Replace `var(--status-*-subtle)` callsites with `color-mix(in oklab, var(--status-*) 18%, var(--surface))` where the design-system intent is the subtle tint, or keep the alias by defining local `--status-fresh-soft: color-mix(...)` etc. — **choose option B** (local aliases) to keep diffs small and avoid touching ~40 call sites. Add to `:root`:
    ```scss
    --status-fresh-soft:    color-mix(in oklab, var(--status-fresh)    18%, var(--surface));
    --status-soon-soft:     color-mix(in oklab, var(--status-soon)     18%, var(--surface));
    --status-critical-soft: color-mix(in oklab, var(--status-critical) 18%, var(--surface));
    --status-expired-soft:  color-mix(in oklab, var(--status-expired)  18%, var(--surface));
    ```
    Then global replace `var(--status-fresh-subtle)` → `var(--status-fresh-soft)` (and 3 siblings), `var(--accent-subtle)` → `var(--accent-soft)`, `var(--bg-subtle)` → `var(--surface-warm)`, `var(--accent-fg)` → `var(--accent-on)`, `var(--fg-3)` → `var(--meta)`, `var(--border-subtle)` → `var(--border)`, `var(--shadow-*)` → `var(--elev-*)`.

11. **Lines 2680, 2688, 2899–2902, 2923, 3017 (status-nodate):** drop `.status-nodate` / `.dot-nodate` / `.active.status-nodate` rules. Update `expiryStatusClass` Go template helper or partials that emit `status-nodate` → map to `status-fresh` (closest neutral) and note in deviations.md. Search templates for `status-nodate` literal usage and update.

### `src/templates/scss/login.scss`

1. **Line 28 (`.login-brand { background: var(--accent); }`):** change to `var(--surface-warm)`.
2. **Lines 30–60 (decorative `::before`, `::after`, `.brand-circle-sm`):** remove the rgba blob decorations. The design-system brand panel is "Logo oben links, Headline zentriert vertikal, Bildplatzhalter 16:9 darunter" — no gradient blobs.
3. **Lines 78–94 (`.brand-logo-mark`, `.brand-logo-name`):** rewrite for dark text on warm crème:
   - `.brand-logo-mark { color: var(--accent); border: 1px solid var(--border); background: var(--surface); }`
   - `.brand-logo-name { color: var(--fg); }`
4. **Lines 96–110 (`.brand-headline`, `.brand-sub`):** use design-system tokens. `color: var(--fg)` for headline, `color: var(--muted)` for sub. Sizes: `font-size: var(--text-display)` and `var(--text-base)` (already close).
5. **Lines 120–145 (`.brand-feature-icon`, `.brand-feature-text`):** swap to dark-text-on-crème palette.
6. **Lines 147–153 (`.brand-bottom`):** `color: var(--meta)`.
7. **Lines 161–164 (`.login-form-panel`):** BG stays `var(--bg)`, padding stays.
8. **Line 181 (`.invite-banner`):** `var(--accent-subtle)` → `var(--accent-soft)`. `oklch(78% .08 162deg)` border → `var(--accent)`.
9. **Lines 250–265 (`.field`, `.field-label`):** unchanged structurally. Tokens only.
10. **Lines 286–312 (`.field-input`):** update oklch focus shadow literal to `var(--focus-ring)`.
11. **Lines 411–446 (`.btn-submit`):** delete and rely on `.btn.btn-primary` (Bootstrap) instead. If the submit button still needs the `width: 100%; height: 44px;` overrides, add a `.login-form-inner .btn-primary { width: 100%; }` rule.

### Template files (`.tmpl`) — full button migration

Search-and-replace across these files:
- `src/templates/web/pages/*.tmpl` (17 files)
- `src/templates/web/partials/*.tmpl` (12 files)
- `src/templates/web/layout/base.tmpl`, `baseAuth.tmpl`

Mapping (preserve any extra classes like `btn-sm`, `btn-lg`, `disabled`):
| Old | New |
|---|---|
| `btn-proviant-primary` | `btn-primary` |
| `btn-proviant-secondary` | `btn-outline-secondary` |
| `btn-proviant-danger` | `btn-danger` |
| `btn-ghost` | `btn-link` |

Also: replace `var(--accent-subtle)`, `var(--accent-fg)`, `var(--bg-subtle)`, `var(--fg-3)`, `var(--border-subtle)`, `var(--brand-slate*)`, `var(--status-nodate*)`, `oklch(...)` literals in inline `style="…"` with the new token names. Examples found: `style="background:var(--accent-subtle);border:1px solid oklch(0.78 0.08 162);"` → `style="background:var(--accent-soft);border:1px solid var(--accent);"`.

### JS files

- `src/assets/js/*.js` — search for `.btn-proviant-`, `.btn-ghost`, `.status-nodate` and update selectors. Affects: likely `products.js`, `settings.js`, `shoppingList.js`, `recipes.js`, etc. — full list pending grep.

### Service worker

- `src/assets/js/sw.js` — bump `CACHE_NAME` (CSS is cache-first per AGENTS.md).

### New file: `design-system/deviations.md`

Document:
- **Tokens dropped:** `--brand-slate*`, `--status-nodate*`, `--color-info: blue` (replaced with `--info: #5A7A8A`).
- **Token renames:** `--accent-subtle` → `--accent-soft`, `--accent-fg` → `--accent-on`, `--bg-subtle` → `--surface-warm`, `--fg-3` → `--meta`, `--border-subtle` → `--border`, `--shadow-{xs,sm,md,lg,xl}` → `--elev-{flat,raised,overlay,…}`.
- **Alias bridge:** local `--status-*-soft` defined as `color-mix(...)` to keep ~40 call sites untouched.
- **Component deferrals (out of scope this pass):** `.app__sidebar/topbar/main`, `.kpi*`, `.toast--*`, `.table-empty`, `.skeleton` (we have `.skeleton-block`), `.modal--demo`, `.toast-stack`, full `.auth` grid, `.page-nav`.
- **Nodate mapping:** `status-nodate` class output maps to `status-fresh` in templates.
- **Card min-height:** kept at 150 px (project rule, not in design-system).
- **Button uppercase + pill:** applied as Bootstrap overrides; design-system also mandates `text-transform: uppercase` on all `.btn`.
- **Type scale:** spec gives 10 named sizes; we expose them as `--text-display/h1/h2/h3/base/base-bold/sm/xs/eyebrow/mono` (renamed to match spec — `--text-md/lg/xl/2xl/3xl/4xl` from old scheme are dropped; project templates use them, so keep them as deprecated aliases pointing at the new tokens).

## Implementation order

1. Add new `:root` token block in `main.scss` (parallel to keeping old aliases initially).
2. Add `--status-*-soft` aliases and Bootstrap mirror vars.
3. Add new component classes (`.eyebrow`, `.stack-*`, `.badge--*`, `.list-group-item__*`, `.field--code`).
4. Replace `var(--accent-subtle)` etc. globally in `main.scss` (the alias bridge means most call sites need no change, but update the few that use direct `oklch(...)` literals).
5. Drop `.status-nodate*` rules; update template partials that emit `status-nodate` class.
6. Rewrite `.btn-proviant-*` block to Bootstrap overrides; add `:focus-visible`, `:disabled`, `[aria-busy]` rules.
7. Run `task css` to compile; visually compare against `design-system/screens/02-dashboard.html` and `01-login.html`.
8. Bulk-replace button classes across `.tmpl` files (one sed / replace pass per file).
9. Bulk-replace token names in `.tmpl` `style="…"` attributes.
10. Update JS selectors.
11. Rewrite `login.scss` to warm-crème brand panel.
12. Bump `CACHE_NAME` in `sw.js`.
13. Run `task lint-css`, `task lint-html`, `task lint-js`.
14. Write `design-system/deviations.md`.
15. Run `task test` to confirm nothing broke (controller tests should be unaffected, but a smoke check is cheap).

## Verification

- `task css` compiles without errors.
- `task lint-css`, `task lint-html`, `task lint-js` pass.
- Visual spot-check of dashboard, products, login pages against `design-system/screens/`:
  - BG is crème (`#FAF7F1`), not the old warmer off-white.
  - Buttons are pill-shaped and uppercase.
  - Primary button is sage `#5A8A7A`, not teal `#2d7858`.
  - Login brand panel is warm crème with dark text (no green BG, no decorative circles).
  - Status chips/badges use subtle tinted backgrounds (`color-mix(18%)`).
  - No `--brand-slate*`, `--status-nodate*`, `--bg-subtle`, `--accent-subtle` references remain in CSS or template inline styles.
- `task test` passes (Go tests).
- Browser console: no missing CSS variable warnings on dashboard / products / login / settings pages.

## Risks & mitigations

- **Token rename breaks template inline styles** if any `style="…var(--accent-subtle)…"` was missed. Mitigation: grep `src/templates/web/**/*.tmpl` for `--accent-subtle|--accent-fg|--bg-subtle|--fg-3|--border-subtle|--brand-slate|--status-nodate` before declaring done.
- **Button migration touches 14+ templates** — risk of a missed button class leaving an unstyled button. Mitigation: grep for `btn-proviant` after the rename and confirm zero hits.
- **`oklch()` color-mix** in old `box-shadow` literals won't compile-fail in Dart Sass, but render slightly differently from the design-system's `rgba(80,60,30,…)` shadows. Mitigation: switch shadow expressions to `var(--elev-*)` tokens (already `rgba` in design-system).
- **VendSans / DMMono font fallback** — the new `--font-display/body` uses `"VendSans"` first; if the woff2 isn't loaded, system-ui takes over. Current code already uses lowercase `vendsans`; the new token uses the case-sensitive name. Mitigation: keep lowercase in the Bootstrap variable `$font-family-base: vendsans, system-ui, sans-serif` and use the same lowercase in `--font-ui` (rename to `--font-display/body` with lowercase string). Adjust the design-system reference in deviations.md if needed.

## Files touched (summary)

- `src/templates/scss/main.scss` — large rewrite (tokens + button block + new components)
- `src/templates/scss/login.scss` — brand panel rewrite
- `src/templates/web/pages/*.tmpl` (17 files) — button + inline-style renames
- `src/templates/web/partials/*.tmpl` (12 files) — same
- `src/templates/web/layout/base.tmpl`, `baseAuth.tmpl` — same
- `src/assets/js/*.js` — selector renames
- `src/assets/js/sw.js` — CACHE_NAME bump
- `design-system/deviations.md` — new file
