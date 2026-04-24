# Proviant — Product List (List View) — Claude Code Implementation Prompt

Paste this prompt directly into Claude Code in your project root.

---

## Prompt

I'm redesigning the product list page in Proviant. Read the design reference at `design-system/ui_kits/proviant/product-list-redesign.html` — switch to the list view (≡ toggle) to see the approved layout. Also read `design-system/colors_and_type.css` for all tokens.

### What to implement

**1. Page header with urgent alert**

Replace the current page header with this pattern:

```html
<!-- templates/products.html -->
<div class="d-flex align-items-center justify-content-between mb-0 px-4 py-3 bg-white border-bottom">
  <div>
    <h1 style="font-size:18px;font-weight:700;letter-spacing:-0.02em;color:var(--fg);">Products</h1>
    <p style="font-size:11px;color:var(--fg-3);margin:0;">{{ .ProductCount }} items in your household</p>
  </div>
  {{ if gt (add .ExpiredCount .CriticalCount) 0 }}
  <div style="background:oklch(0.95 0.05 25);border:1px solid oklch(0.80 0.12 25);border-radius:8px;padding:8px 12px;display:flex;align-items:center;gap:8px;font-size:12px;">
    <i class="bi bi-exclamation-circle-fill" style="color:oklch(0.55 0.20 25);font-size:14px;"></i>
    <span style="color:oklch(0.40 0.18 25);font-weight:500;">{{ .ExpiredCount }} expired · {{ .CriticalCount }} critical</span>
    <a href="?status=expired" style="color:oklch(0.45 0.20 25);font-size:11px;font-weight:600;">Show expired →</a>
  </div>
  {{ end }}
</div>
```

**2. Toolbar — single search + status filters + location filters**

Replace the current two-field search and location-only filters:

```html
<div style="background:white;border-bottom:1px solid var(--border);padding:10px 16px;display:flex;flex-direction:column;gap:8px;">
  <!-- Row 1: search + view modes + add button -->
  <div class="d-flex gap-2 align-items-center">
    <div style="flex:1;position:relative;">
      <i class="bi bi-search" style="position:absolute;left:10px;top:50%;transform:translateY(-50%);color:var(--fg-3);font-size:14px;"></i>
      <input type="text" name="q" value="{{ .Query }}"
        placeholder="Search products…"
        style="font-family:var(--font-ui);font-size:13px;color:var(--fg);background:var(--bg-subtle);border:1px solid var(--border);border-radius:8px;padding:8px 12px 8px 32px;width:100%;outline:none;">
    </div>
    <a href="?{{ queryWith .Params "view" "list" }}" class="btn-view {{ if eq .View "list" }}active{{ end }}">
      <i class="bi bi-list-ul"></i>
    </a>
    <a href="?{{ queryWith .Params "view" "grid" }}" class="btn-view {{ if eq .View "grid" }}active{{ end }}">
      <i class="bi bi-grid-3x3-gap"></i>
    </a>
    <a href="/products/new" class="btn btn-proviant-primary" style="white-space:nowrap;">
      <i class="bi bi-plus-lg"></i> Add product
    </a>
  </div>

  <!-- Row 2: status filters + location filters + bulk actions -->
  <div class="d-flex gap-1 align-items-center flex-wrap">
    <!-- Status filter pills -->
    {{ range $s := slice "all" "expired" "critical" "soon" "fresh" "nodate" }}
    <a href="?{{ queryWith $.Params "status" $s }}"
      class="filter-pill {{ if eq $.StatusFilter $s }}active status-{{ $s }}{{ end }}">
      {{ if ne $s "all" }}<span class="filter-dot dot-{{ $s }}"></span>{{ end }}
      {{ statusLabel $s }}
    </a>
    {{ end }}

    <div style="width:1px;height:16px;background:var(--border);margin:0 4px;"></div>

    <!-- Location filter pills -->
    {{ range $l := .StorageLocations }}
    <a href="?{{ queryWith $.Params "location" $l.Name }}"
      class="filter-pill {{ if eq $.LocationFilter $l.Name }}active{{ end }}">
      {{ $l.Name }}
    </a>
    {{ end }}
    <a href="?{{ queryWith $.Params "location" "" }}"
      class="filter-pill {{ if eq $.LocationFilter "" }}active{{ end }}">All</a>

    <!-- Bulk actions (shown when items selected via JS) -->
    <div id="bulkActions" style="margin-left:auto;display:none;gap:6px;align-items:center;">
      <span id="bulkCount" style="font-size:11px;color:var(--accent);font-weight:600;"></span>
      <button onclick="bulkAction('edit')" class="btn-bulk">
        <i class="bi bi-pencil"></i> Edit
      </button>
      <button onclick="bulkAction('archive')" class="btn-bulk">
        <i class="bi bi-archive"></i> Archive
      </button>
      <button onclick="bulkAction('delete')" class="btn-bulk danger">
        <i class="bi bi-trash3"></i> Delete
      </button>
    </div>
  </div>
</div>
```

**3. List view table**

Replace the card grid with a compact table:

```html
<!-- List view -->
<div id="productList">
  <!-- Column headers -->
  <div class="list-header">
    <div style="width:32px;">
      <input type="checkbox" id="selectAll" onchange="toggleSelectAll(this)"
        style="width:15px;height:15px;accent-color:var(--accent);cursor:pointer;">
    </div>
    <div class="col-product">Product</div>
    <div class="col-status">Status</div>
    <div class="col-expiry">Expiry</div>
    <div class="col-location">Location</div>
    <div class="col-qty">Qty</div>
  </div>

  <!-- Product rows -->
  {{ range .Products }}
  <div class="list-row" data-id="{{ .ID }}">
    <!-- Checkbox -->
    <div style="width:32px;">
      <input type="checkbox" class="row-checkbox" value="{{ .ID }}"
        onchange="updateBulkSelection()"
        style="width:15px;height:15px;accent-color:var(--accent);cursor:pointer;">
    </div>

    <!-- Product name + thumbnail -->
    <div class="col-product">
      <a href="/products/{{ .ID }}/edit" class="product-thumb" style="position:relative;">
        {{ if .ImageURL }}
        <img src="{{ .ImageURL }}" alt="{{ .Name }}"
          style="width:32px;height:32px;border-radius:6px;object-fit:contain;background:var(--bg-subtle);">
        {{ else }}
        <div style="width:32px;height:32px;border-radius:6px;background:var(--bg-subtle);display:flex;align-items:center;justify-content:center;">
          <i class="bi bi-box-seam" style="font-size:14px;color:var(--fg-3);"></i>
        </div>
        {{ end }}
        <!-- Status dot on thumbnail -->
        <div style="position:absolute;bottom:-2px;right:-2px;width:10px;height:10px;border-radius:999px;background:var(--status-{{ .Status }}-dot);border:1.5px solid white;"></div>
      </a>
      <div style="min-width:0;">
        <a href="/products/{{ .ID }}/edit"
          style="font-size:13px;font-weight:600;color:var(--fg);text-decoration:none;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;display:block;">
          {{ .Name }}
        </a>
        <div style="font-size:10px;color:var(--fg-3);font-family:var(--font-mono);">
          #{{ .ID }} · {{ .Category }}
        </div>
      </div>
    </div>

    <!-- Status badge -->
    <div class="col-status">
      <span class="badge-status badge-{{ .Status }}">
        <i class="bi bi-{{ statusIcon .Status }}"></i>
        {{ statusLabel .Status }}
      </span>
    </div>

    <!-- Expiry (relative) -->
    <div class="col-expiry" style="font-family:var(--font-mono);font-size:11px;color:{{ expiryColor .Status }};">
      {{ .ExpiryRelative }}
    </div>

    <!-- Location -->
    <div class="col-location" style="font-size:11px;color:var(--fg-3);">
      {{ if .StorageLocation }}{{ .StorageLocation }}{{ else }}—{{ end }}
    </div>

    <!-- Amount stepper -->
    <div class="col-qty">
      <div class="qty-stepper">
        <button type="button" onclick="changeQty({{ .ID }}, -1)">−</button>
        <span id="qty-{{ .ID }}">{{ .Amount }}</span>
        <button type="button" onclick="changeQty({{ .ID }}, 1)">+</button>
      </div>
    </div>
  </div>
  {{ end }}

  <!-- Empty state -->
  {{ if not .Products }}
  <div style="padding:48px 24px;text-align:center;">
    <div style="width:48px;height:48px;background:var(--bg-subtle);border-radius:10px;display:flex;align-items:center;justify-content:center;margin:0 auto 14px;font-size:22px;color:var(--fg-3);">
      <i class="bi bi-search"></i>
    </div>
    <div style="font-size:14px;font-weight:600;color:var(--fg);margin-bottom:6px;">No results found</div>
    <div style="font-size:12px;color:var(--fg-2);margin-bottom:16px;">Try adjusting your search or filters</div>
    <a href="/products" class="btn btn-proviant-secondary btn-sm">Clear filters</a>
  </div>
  {{ end }}
</div>
```

**4. CSS to add**

Add to your main stylesheet after the Proviant base tokens:

```css
/* ── Product list ───────────────────────────────────── */
.list-header {
  display: grid;
  grid-template-columns: 32px 1fr 120px 140px 90px 80px;
  gap: 10px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--border);
  background: var(--bg-subtle);
}
.list-header > div {
  font-size: 10px; font-weight: 600; text-transform: uppercase;
  letter-spacing: 0.06em; color: var(--fg-3);
}
.list-row {
  display: grid;
  grid-template-columns: 32px 1fr 120px 140px 90px 80px;
  gap: 10px;
  padding: 9px 16px;
  align-items: center;
  border-bottom: 1px solid oklch(0.93 0.006 75);
  transition: background 120ms;
  cursor: default;
}
.list-row:hover { background: oklch(0.97 0.006 75); }
.list-row.selected { background: oklch(0.50 0.12 162 / 0.06); }

.col-product { display: flex; align-items: center; gap: 10px; min-width: 0; }
.col-status  { }
.col-expiry  { }
.col-location{ }
.col-qty     { }

/* Status dot CSS vars */
--status-expired-dot:  oklch(0.55 0.20 25);
--status-critical-dot: oklch(0.64 0.18 45);
--status-soon-dot:     oklch(0.72 0.16 80);
--status-fresh-dot:    oklch(0.50 0.12 162);
--status-nodate-dot:   oklch(0.60 0.010 70);

/* Qty stepper */
.qty-stepper {
  display: flex; align-items: center;
  background: white; border: 1px solid var(--border);
  border-radius: 6px; overflow: hidden; width: fit-content;
}
.qty-stepper button {
  width: 22px; height: 26px; background: var(--bg-subtle); border: none;
  cursor: pointer; color: var(--fg-2); font-size: 13px;
  display: flex; align-items: center; justify-content: center;
  transition: background 120ms;
}
.qty-stepper button:hover { background: var(--border); }
.qty-stepper span {
  font-family: var(--font-mono); font-size: 12px; font-weight: 600;
  color: var(--fg); width: 24px; text-align: center;
}

/* Filter pills */
.filter-pill {
  padding: 4px 10px; border-radius: 999px;
  border: 1px solid var(--border); background: white;
  color: var(--fg-2); font-family: var(--font-ui); font-size: 11px;
  text-decoration: none; display: inline-flex; align-items: center; gap: 4px;
  transition: all 120ms; white-space: nowrap;
}
.filter-pill:hover { background: var(--bg-subtle); color: var(--fg); }
.filter-pill.active { border-width: 1.5px; font-weight: 600; }
.filter-pill.active.status-all      { border-color: var(--accent); background: var(--accent-subtle); color: var(--accent); }
.filter-pill.active.status-expired  { border-color: oklch(0.80 0.12 25); background: oklch(0.95 0.05 25); color: oklch(0.45 0.20 25); }
.filter-pill.active.status-critical { border-color: oklch(0.82 0.12 45); background: oklch(0.96 0.05 45); color: oklch(0.50 0.18 45); }
.filter-pill.active.status-soon     { border-color: oklch(0.85 0.10 80); background: oklch(0.97 0.05 80); color: oklch(0.50 0.16 80); }
.filter-pill.active.status-fresh    { border-color: oklch(0.78 0.08 145); background: oklch(0.94 0.04 145); color: oklch(0.38 0.10 145); }
.filter-pill.active.status-nodate   { border-color: oklch(0.80 0.008 70); background: oklch(0.94 0.006 70); color: oklch(0.45 0.010 70); }

.filter-dot { width: 6px; height: 6px; border-radius: 999px; display: inline-block; }
.dot-expired  { background: oklch(0.55 0.20 25); }
.dot-critical { background: oklch(0.64 0.18 45); }
.dot-soon     { background: oklch(0.72 0.16 80); }
.dot-fresh    { background: oklch(0.50 0.12 162); }
.dot-nodate   { background: oklch(0.60 0.010 70); }

/* View toggle buttons */
.btn-view {
  width: 32px; height: 32px; border-radius: 7px; border: 1px solid var(--border);
  background: white; color: var(--fg-3); display: inline-flex; align-items: center;
  justify-content: center; font-size: 15px; text-decoration: none; transition: all 120ms;
}
.btn-view:hover, .btn-view.active { background: var(--accent-subtle); border-color: oklch(0.78 0.08 162); color: var(--accent); }

/* Bulk action buttons */
.btn-bulk {
  background: white; border: 1px solid var(--border); border-radius: 7px;
  padding: 5px 10px; font-family: var(--font-ui); font-size: 11px;
  cursor: pointer; display: inline-flex; align-items: center; gap: 5px; color: var(--fg-2);
}
.btn-bulk.danger {
  background: oklch(0.95 0.05 25); border-color: oklch(0.80 0.12 25); color: oklch(0.45 0.20 25);
}

/* Sidebar badge */
.nav-badge {
  font-size: 9px; font-weight: 700; background: oklch(0.55 0.20 25);
  color: white; border-radius: 999px; padding: 1px 5px; min-width: 16px; text-align: center;
}
```

**5. Plain JS — checkbox selection + qty stepper + AJAX qty update**

```javascript
// product-list.js

// ── Checkbox selection ──────────────────────────────
function updateBulkSelection() {
  const checked = document.querySelectorAll('.row-checkbox:checked');
  const bulk    = document.getElementById('bulkActions');
  const count   = document.getElementById('bulkCount');
  const rows    = document.querySelectorAll('.list-row');

  // Update row highlight
  rows.forEach(row => {
    const cb = row.querySelector('.row-checkbox');
    row.classList.toggle('selected', cb && cb.checked);
  });

  // Show/hide bulk action bar
  if (checked.length > 0) {
    bulk.style.display = 'flex';
    count.textContent  = checked.length + ' selected';
  } else {
    bulk.style.display = 'none';
  }

  // Sync "select all" checkbox state
  const all = document.getElementById('selectAll');
  const total = document.querySelectorAll('.row-checkbox').length;
  all.indeterminate = checked.length > 0 && checked.length < total;
  all.checked       = checked.length === total;
}

function toggleSelectAll(cb) {
  document.querySelectorAll('.row-checkbox').forEach(c => c.checked = cb.checked);
  updateBulkSelection();
}

function bulkAction(action) {
  const ids = [...document.querySelectorAll('.row-checkbox:checked')].map(c => c.value);
  if (!ids.length) return;
  if (action === 'delete' && !confirm(`Delete ${ids.length} product(s)?`)) return;

  fetch(`/products/bulk-${action}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ids, csrf_token: document.querySelector('[name=csrf_token]').value }),
  }).then(() => window.location.reload());
}

// ── Qty stepper ─────────────────────────────────────
function changeQty(id, delta) {
  const span = document.getElementById('qty-' + id);
  if (!span) return;
  const newVal = Math.max(0, parseInt(span.textContent) + delta);
  span.textContent = newVal;

  // AJAX update — adjust endpoint to match your Go routes
  fetch(`/products/${id}/qty`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ amount: newVal, csrf_token: document.querySelector('[name=csrf_token]').value }),
  });
}
```

**6. Go template helpers needed**

Add these template funcs to your Go handler:

```go
// Status label
func statusLabel(s string) string {
    labels := map[string]string{
        "expired":  "Expired",
        "critical": "Critical",
        "soon":     "Expiring soon",
        "fresh":    "Fresh",
        "nodate":   "No date",
        "all":      "All",
    }
    return labels[s]
}

// Status icon (Bootstrap Icons)
func statusIcon(s string) string {
    icons := map[string]string{
        "expired":  "x-circle-fill",
        "critical": "exclamation-circle-fill",
        "soon":     "clock-fill",
        "fresh":    "check-circle-fill",
        "nodate":   "dash-circle-fill",
    }
    return icons[s]
}

// Expiry text color (inline CSS value)
func expiryColor(s string) string {
    colors := map[string]string{
        "expired":  "oklch(0.45 0.20 25)",
        "critical": "oklch(0.50 0.18 45)",
        "soon":     "oklch(0.50 0.16 80)",
        "fresh":    "var(--fg-2)",
        "nodate":   "var(--fg-3)",
    }
    return colors[s]
}

// URL param helper
func queryWith(params url.Values, key, val string) string {
    p := url.Values{}
    for k, v := range params { p[k] = v }
    if val == "" { p.Del(key) } else { p.Set(key, val) }
    return p.Encode()
}
```

### Do not change

- Any database or model code
- The product struct fields
- Existing POST routes for archive/delete (bulk-action endpoints are new additions)
- Any authentication middleware

### Verify when done

1. List view renders with 6 columns: checkbox, product, status, expiry, location, qty
2. Status filter pills filter the list — each status color matches the design tokens
3. Expired/critical alert banner appears in page header when items exist
4. Selecting rows shows the bulk action bar with count
5. Select all / deselect all works
6. Qty stepper sends AJAX request to `/products/:id/qty`
7. Sidebar Products nav item shows a red badge count of expired + critical items
8. Empty state renders when no products match the filter
