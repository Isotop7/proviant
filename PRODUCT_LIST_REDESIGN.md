# Plan: Product List Redesign

## Context

Implementing the approved product list redesign from `design-system/PRODUCT_LIST_HANDOFF.md`. Goal: replace the card-only grid with a **list view** (default recommended), add status filter pills, urgent alert banner, qty stepper, bulk-select toolbar, nav badge, and a view toggle (list/grid). Grid view stays intact.

---

## Critical Files

| File | Change |
|------|--------|
| `src/templates/template.go` | Add 2 template funcs: `expiryColor`, `queryWith` |
| `src/web/frontend.go` | Enrich Products handler with status filter, view mode, counts, url.Values |
| `src/templates/web/pages/products.tmpl` | Replace header + toolbar; add list view; keep grid + add-product modal |
| `src/templates/web/partials/nav.tmpl` | Add urgent badge to Products nav item |
| `src/templates/scss/main.scss` | Append list-view CSS block |
| `src/assets/js/products.js` | Append new list-view JS (bulk select, qty stepper) |

---

## Step-by-Step Implementation

### 1 — `src/templates/template.go`

Add to `customTemplateFunctions` map and implement:

```go
func expiryColor(t time.Time) string {
    switch expiryStatusClass(t) {
    case "expired":  return "oklch(0.45 0.20 25)"
    case "critical": return "oklch(0.50 0.18 45)"
    case "soon":     return "oklch(0.50 0.16 80)"
    case "fresh":    return "var(--fg-2)"
    default:         return "var(--fg-3)"
    }
}

func queryWith(params url.Values, key, val string) string {
    p := url.Values{}
    for k, v := range params { p[k] = v }
    if val == "" { p.Del(key) } else { p.Set(key, val) }
    return p.Encode()
}
```

Add `"net/url"` to imports. Register both in `customTemplateFunctions`.

### 2 — `src/web/frontend.go`

In the `Products` handler, after fetching products:

1. Parse new params:
   - `view := ctx.DefaultQuery("view", "grid")`
   - `statusFilter := ctx.DefaultQuery("status", "all")`
2. Compute counts by ranging over all products using `expiryStatusClassFromTime()` logic (inline — no new func needed):
   ```go
   expiredCount, criticalCount := 0, 0
   for _, p := range products {
       switch templates.StatusClass(p.ExpireAt) { ... }
   }
   ```
   Export `expiryStatusClass` logic OR compute inline:
   ```go
   now := time.Now()
   for _, p := range products {
       if !p.ExpireAt.IsZero() && p.ExpireAt.Before(now) {
           expiredCount++
       } else if !p.ExpireAt.IsZero() && p.ExpireAt.Before(now.Add(3*24*time.Hour)) {
           criticalCount++
       }
   }
   ```
3. Filter products by `statusFilter` (after computing counts on full set):
   ```go
   if statusFilter != "all" {
       var filtered []dbModel.Product
       for _, p := range products {
           if computedStatus(p.ExpireAt) == statusFilter {
               filtered = append(filtered, p)
           }
       }
       products = filtered
   }
   ```
4. Build `url.Values` from current request:
   ```go
   params := url.Values{}
   for k, v := range ctx.Request.URL.Query() { params[k] = v }
   ```
5. Extend `pageData`:
   ```go
   "View":           view,
   "StatusFilter":   statusFilter,
   "ProductCount":   len(allProducts),  // before status filter
   "ExpiredCount":   expiredCount,
   "CriticalCount":  criticalCount,
   "UrgentCount":    expiredCount + criticalCount,
   "Params":         params,
   "StorageLocations": locations,  // alias (keep "Locations" too for modal compatibility)
   ```

**Note:** `expiryStatusClass` in template.go takes `time.Time`. Inline the same logic in the handler. Don't expose a public func — keep DRY via the template func for templates, inline for Go.

### 3 — `src/templates/web/pages/products.tmpl`

Keep the entire add-product modal (lines 55–316) and script tags **unchanged**.

Replace only the top section (lines 1–53):

**3a. Page header** — replace `.page-header` div with the handoff pattern:
- Title + `{{ .ProductCount }} items` subtitle
- Conditional alert banner when `{{ gt (add .ExpiredCount .CriticalCount) 0 }}` — but Go templates don't have `add`; use `{{ if gt .UrgentCount 0 }}` instead

**3b. Toolbar** — replace `.toolbar-sticky` with the 2-row toolbar:
- Row 1: search input (`name="queryValue"` preserving existing search), view toggle links, Add product button
- Row 2: status filter pills (range over statuses), separator, location filter pills (range `.StorageLocations`), bulk action bar div (hidden by default, shown via JS)

**3c. Product list section** — replace the grid with a conditional:
```html
{{ if eq .View "list" }}
  <!-- list view: header row + product rows -->
{{ else }}
  <!-- existing grid -->
{{ end }}
{{ template "noproducts" . }}
```

**Field mappings in list view template:**
| Handoff field | Template expression |
|---|---|
| `.Name` | `.ProductName` |
| `.Status` (string) | `{{ expiryStatusClass .ExpireAt }}` |
| `.ExpiryRelative` | `{{ expiryUrgencyText .ExpireAt }}` (or `—` when empty) |
| `.StorageLocation` (string) | `{{ if .StorageLocation }}{{ .StorageLocation.Name }}{{ else }}—{{ end }}` |
| `statusLabel .Status` | `{{ expiryStatusLabel .ExpireAt }}` |
| `statusIcon .Status` | `{{ expiryStatusIcon .ExpireAt }}` |
| `expiryColor .Status` | `{{ expiryColor .ExpireAt }}` |
| `.ImageURL` | `.ImageURL` ✓ |
| `.Amount` | `.Amount` ✓ |
| `.ID` | `.ID` ✓ |

**Status dot CSS var** — use inline style: `background:var(--status-{{ expiryStatusClass .ExpireAt }}-dot)` — these vars get added in step 5.

**changeQty JS**: The list view calls `changeQty(id, delta)` — delta ±1, calling existing `proviant.updateProductAmount()`.

**Qty stepper JS call**: write as `onclick="changeQty({{ .ID }}, -1)"` and `changeQty({{ .ID }}, 1)`.

### 4 — `src/templates/web/partials/nav.tmpl`

On the Products `<a>` element, add badge after the text:
```html
{{ if gt .UrgentCount 0 }}
<span class="nav-badge">{{ .UrgentCount }}</span>
{{ end }}
```
On other pages `.UrgentCount` is 0/missing → badge hidden.

### 5 — `src/templates/scss/main.scss`

Append the full CSS block from handoff section 4, with two adjustments:
1. Move the `--status-*-dot` CSS vars into `:root` (they must be declared as variables, not standalone rules)
2. Keep existing `.badge-status` and status color vars — no conflicts

### 6 — `src/assets/js/products.js`

Append to the existing file (don't replace — existing grid-view logic must stay):

```javascript
// ── List view: checkbox selection ──────────────────────────────
function updateBulkSelection() { ... }
function toggleSelectAll(cb) { ... }
function bulkAction(action) {
  const ids = [...document.querySelectorAll('.row-checkbox:checked')].map(c => +c.value);
  if (!ids.length) return;
  if (action === 'delete') {
    if (!confirm(`Delete ${ids.length} product(s)?`)) return;
    proviant.bulkDeleteProducts(ids).then(() => window.location.reload());
  } else if (action === 'archive') {
    proviant.bulkArchiveProducts(ids).then(() => window.location.reload());
  }
}

// ── List view: qty stepper ──────────────────────────────────────
function changeQty(id, delta) {
  const span = document.getElementById('qty-' + id);
  if (!span) return;
  const newVal = Math.max(0, parseInt(span.textContent) + delta);
  span.textContent = newVal;
  proviant.updateProductAmount(id, delta);
}
```

Use `proviant.bulkDeleteProducts` / `proviant.bulkArchiveProducts` / `proviant.updateProductAmount` — these exist in `src/assets/js/proviant.js` and handle JWT auth.

---

## Tricky Details

- `queryWith` helper needs `net/url` import in template.go
- `add` template func doesn't exist — use `.UrgentCount` (precomputed in handler) instead of `{{ add .ExpiredCount .CriticalCount }}` in template
- `expiryUrgencyText` returns `""` for fresh items far in the future — show `{{ expiryUrgencyText .ExpireAt | default "—" }}` or use `{{ if . }}{{ . }}{{ else }}—{{ end }}` pattern with a pipeline: `{{ with expiryUrgencyText .ExpireAt }}{{ . }}{{ else }}—{{ end }}`
- Status filter must compute counts **before** filtering the products slice
- Existing `LocationFilter` is an ID string; new toolbar needs location **name** to match `.StorageLocations` range — either switch to name-based filtering (breaking change) or keep ID-based and use `{{ printf "%d" .ID }}` comparison in template
- **CSS cache**: bump `CACHE_NAME` in `src/assets/js/sw.js` after CSS change (per CLAUDE.md)

---

## Verification Checklist

1. `task css` — no SCSS compile errors
2. `task tidy` — no Go format/mod errors  
3. `task test` — all tests pass
4. Start server with `task run`, open `/web/products`
5. List view renders with 6 columns: checkbox, product, status, expiry, location, qty
6. Status filter pills change the list; each status color matches design tokens
7. Alert banner appears in page header when expired/critical items exist
8. Selecting rows shows bulk action bar with count
9. Select all / deselect all works
10. Qty stepper ± buttons fire AJAX (check Network tab: `PATCH /api/v1/products/:id/amount`)
11. Bulk delete calls `DELETE /api/v1/products/bulkDelete` (existing endpoint)
12. Grid view still works (toggle back)
13. Nav badge shows correct count on all pages (Products handler passes it; other pages show 0 → no badge)
14. Empty state renders when no products match filter
