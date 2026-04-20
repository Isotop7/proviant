# Proviant UI Refactor — Claude Code Handoff Brief

## Context
This is a refactoring task. The goal is to update the existing Proviant UI to match the
Proviant design system. Do not change any backend logic, routing, Go template data bindings,
or functionality — only update HTML structure, CSS classes, and static assets.

## Design system location
All design system files are in `design-system/` (or wherever you placed the downloaded folder).
Start by reading these files in order:
1. `design-system/README.md` — brand context, visual rules, do/don't guidance
2. `design-system/colors_and_type.css` — every CSS token (color, type, spacing, shadow, radius)
3. `design-system/ui_kits/proviant/bootstrap-snippets.html` — reference HTML for every component

## Step 1 — Add the CSS foundation

Add to your base template `<head>` (after Bootstrap CSS):

```html
<link rel="stylesheet"
  href="https://fonts.googleapis.com/css2?family=Vend+Sans:wght@300;400;500;600;700&family=DM+Mono:wght@400;500&display=swap">
<link rel="stylesheet" href="/static/css/proviant.css">
```

Copy `design-system/colors_and_type.css` → `static/css/proviant.css` in your repo.

Also copy:
- `design-system/assets/icon.svg`     → `static/img/icon.svg`
- `design-system/assets/header.png`   → `static/img/header.png`

---

## Step 2 — Global base styles

In `proviant.css` or your main stylesheet, add/update:

```css
body {
  font-family: var(--font-ui);
  font-size: var(--text-base);
  color: var(--fg);
  background: var(--bg);
  -webkit-font-smoothing: antialiased;
}
```

---

## Step 3 — Replace buttons

Find all `<button>` and `<a class="btn ...">` elements and remap classes:

| Old class | New class |
|-----------|-----------|
| `btn-primary` | `btn btn-proviant-primary` |
| `btn-secondary`, `btn-outline-*` | `btn btn-proviant-secondary` |
| `btn-danger`, `btn-outline-danger` | `btn btn-proviant-danger` |

Add button CSS from `bootstrap-snippets.html` `<style>` block (`.btn-proviant-*` rules).

---

## Step 4 — Replace status badges

Proviant has exactly 5 expiry states. Update your Go templates to output the correct badge class:

```html
<!-- In your Go template -->
<span class="badge-status badge-{{ .Product.Status }}">
  <!-- Status is one of: expired | critical | soon | fresh | nodate -->
  <i class="bi bi-{{ statusIcon .Product.Status }}"></i>
  {{ .Product.StatusLabel }}
</span>
```

Add the `.badge-status` + `.badge-expired/critical/soon/fresh/nodate` CSS from `bootstrap-snippets.html`.

Status → icon mapping:
| Status | Icon class | Label |
|--------|-----------|-------|
| `expired` | `bi-x-circle-fill` | Expired |
| `critical` | `bi-exclamation-circle-fill` | Critical |
| `soon` | `bi-clock-fill` | Expiring soon |
| `fresh` | `bi-check-circle-fill` | Fresh |
| `nodate` | `bi-dash-circle-fill` | No date |

**Important:** Status must always include both the colored badge AND the text label — never
color alone (accessibility requirement).

---

## Step 5 — Update product cards

Each product card should follow this structure (see `bootstrap-snippets.html` for full HTML):

```html
<div class="card product-card">
  <!-- 3px status color bar at top -->
  <div class="status-bar" style="background: var(--status-{{ .Product.Status }}-bar);"></div>
  <!-- Product image or striped placeholder -->
  <div class="product-img">
    {{ if .Product.ImageURL }}
      <img src="{{ .Product.ImageURL }}" alt="{{ .Product.Name }}">
    {{ end }}
  </div>
  <div class="card-body p-3">
    <div class="product-name">{{ .Product.Name }}</div>
    <div class="product-cat mb-2">{{ .Product.Category }}</div>
    <span class="badge-status badge-{{ .Product.Status }}">...</span>
    <div class="product-meta mt-2">
      <i class="bi bi-calendar-event"></i> {{ .Product.ExpiryFormatted }}
    </div>
  </div>
</div>
```

Add the `.product-card`, `.status-bar`, `.product-img`, `.product-name`, `.product-cat`,
`.product-meta` CSS from `bootstrap-snippets.html`.

Status bar colors to add as CSS vars or inline:
```css
--status-expired-bar:  oklch(0.55 0.20 25);
--status-critical-bar: oklch(0.64 0.18 45);
--status-soon-bar:     oklch(0.72 0.16 80);
--status-fresh-bar:    oklch(0.50 0.12 162);
--status-nodate-bar:   oklch(0.60 0.010 70);
```

---

## Step 6 — Update forms

Replace form control styles. The `proviant.css` tokens already override Bootstrap's
`:focus` ring. Additionally update labels:

```html
<label class="form-label" style="font-size:12px; font-weight:500;">
  Field name <span style="color:var(--status-expired)">*</span>
</label>
<input type="text" class="form-control" placeholder="...">
<!-- Errors: -->
<div class="invalid-feedback">
  <i class="bi bi-exclamation-circle-fill"></i> Error message here.
</div>
```

---

## Step 7 — Update sidebar / nav

Replace existing nav with the `.pv-sidebar` / `.pv-nav-item` pattern:

```html
<nav class="pv-sidebar">
  <div class="brand">
    <img src="/static/img/header.png" alt="Proviant">
  </div>
  <div class="nav-items">
    <a href="/dashboard" class="pv-nav-item {{ if eq .Page "dashboard" }}active{{ end }}">
      <i class="bi bi-speedometer2"></i> Dashboard
    </a>
    <a href="/products" class="pv-nav-item {{ if eq .Page "products" }}active{{ end }}">
      <i class="bi bi-box-seam"></i> Products
    </a>
    <a href="/archive" class="pv-nav-item {{ if eq .Page "archive" }}active{{ end }}">
      <i class="bi bi-archive"></i> Archive
    </a>
    <a href="/settings" class="pv-nav-item {{ if eq .Page "settings" }}active{{ end }}">
      <i class="bi bi-person-circle"></i> Settings
    </a>
  </div>
</nav>
```

---

## Step 8 — Update Bootstrap modals

Modals already use Bootstrap 5 JS — only the styling needs updating.
Add to your stylesheet (from `bootstrap-snippets.html`):

```css
.modal-content { border-radius: var(--radius-lg); box-shadow: var(--shadow-xl); }
.modal-header  { border-bottom: 1px solid var(--border-subtle); }
.modal-footer  { border-top:    1px solid var(--border-subtle); }
.modal-icon    { width:40px; height:40px; border-radius:10px; display:flex; align-items:center; justify-content:center; font-size:18px; }
```

Use `.btn-proviant-*` classes on modal action buttons (Step 3).

---

## Step 9 — Update toasts

Add the `.pv-toast` classes and update toast HTML:

```html
<div class="toast pv-toast toast-success" ...>
  <div class="toast-body d-flex align-items-center gap-2">
    <i class="bi bi-check-circle-fill" style="color:oklch(0.50 0.12 162);font-size:16px;"></i>
    <span style="font-size:13px; flex:1;">{{ .Message }}</span>
    <button type="button" class="btn-close" data-bs-dismiss="toast"></button>
  </div>
</div>
```

Classes: `toast-success` | `toast-error` | `toast-warning`

---

## Step 10 — Update dashboard metric tiles

```html
<div class="metric-tile">
  <div class="metric-label">
    <i class="bi bi-box-seam"></i> Total products
  </div>
  <div class="metric-value" style="color:var(--fg);">{{ .Stats.Total }}</div>
</div>
```

---

## Step 11 — Empty states

Any list/grid with no results should render:

```html
<div class="text-center py-5">
  <div style="width:52px;height:52px;background:var(--bg-subtle);border-radius:12px;
              display:flex;align-items:center;justify-content:center;
              margin:0 auto 14px;font-size:24px;color:var(--fg-3);">
    <i class="bi bi-box-seam"></i>
  </div>
  <div style="font-size:14px;font-weight:600;color:var(--fg);margin-bottom:6px;">No products yet</div>
  <div style="font-size:13px;color:var(--fg-2);margin-bottom:18px;">
    Add your first product to start tracking expiry dates.
  </div>
  <a href="/products/new" class="btn btn-proviant-primary btn-sm">
    <i class="bi bi-plus-lg"></i> Add product
  </a>
</div>
```

---

## Design rules to enforce (do not break these)

- **Never convey status with color alone** — always pair with badge text label
- **Font:** `var(--font-ui)` (Vend Sans) everywhere; `var(--font-mono)` for dates, barcodes, codes only
- **Spacing:** use `var(--space-*)` tokens or Bootstrap spacing utilities; do not add new arbitrary px values
- **Icons:** Bootstrap Icons only (`bi-*`); no emoji, no custom SVG icons
- **Borders:** `var(--border)` for default, `var(--accent)` for focus rings — never hardcoded colors
- **Backgrounds:** `var(--bg)` for page, `var(--surface)` (white) for cards/panels — no gradients
- **Corner radius:** `var(--radius-md)` (8px) for cards/inputs/buttons; `var(--radius-lg)` (12px) for modals
- **Shadows:** use `var(--shadow-sm)` for cards (resting), `var(--shadow-md)` on hover, `var(--shadow-xl)` for modals

---

## Files to read before starting

```
design-system/README.md
design-system/colors_and_type.css
design-system/ui_kits/proviant/bootstrap-snippets.html
```

## Files NOT to touch

- Any Go handler/controller files
- Any database or model files
- Any routing configuration
- Existing template data bindings (`.Product.Name`, `.Stats.Total`, etc.)
