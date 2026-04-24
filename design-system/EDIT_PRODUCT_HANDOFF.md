# Proviant — Edit Product Page — Claude Code Implementation Prompt

Paste this prompt directly into Claude Code in your project root.

---

## Prompt

I'm redesigning the edit product page in Proviant. Read the design reference file at `design-system/ui_kits/proviant/edit-product.html` — it's a fully working Bootstrap 5 prototype showing the exact layout, CSS classes, and component patterns to implement. Also read `design-system/colors_and_type.css` for all CSS tokens.

### What to implement

**1. Two-column page layout**

Replace the current flat single-column form with a two-column grid. Copy the `.edit-grid`, `.product-panel`, `.page-wrap` CSS from the reference file.

```html
<!-- templates/edit-product.html -->
<div class="page-wrap">
  <!-- Breadcrumb -->
  <div class="breadcrumb-row">
    <a href="/products"><i class="bi bi-box-seam"></i> Products</a>
    <i class="bi bi-chevron-right"></i>
    <span>Edit product</span>
  </div>

  <!-- Page header -->
  <div class="page-header">
    <div>
      <div class="page-title-row">
        <h1 class="page-title">Edit product</h1>
        <span class="id-chip">#{{ .Product.ID }}</span>
      </div>
      <div class="page-sub">Changes are saved after clicking Update product</div>
    </div>
    <div>
      <span class="badge-status badge-{{ .Product.Status }}">
        <i class="bi bi-{{ statusIcon .Product.Status }}"></i>
        {{ .Product.StatusLabel }}
      </span>
    </div>
  </div>

  <div class="edit-grid">
    <!-- LEFT: form cards -->
    <div>
      <!-- cards go here -->
    </div>
    <!-- RIGHT: product panel -->
    <div class="product-panel">
      <!-- panel goes here -->
    </div>
  </div>
</div>
```

**2. Four grouped form cards**

Replace the flat label+input list with four cards. Copy `.card`, `.card-header`, `.card-header-icon`, `.card-header-title`, `.card-body` CSS from the reference.

**Card 1 — Identity**
```html
<div class="card">
  <div class="card-header">
    <div class="card-header-icon"><i class="bi bi-tag"></i></div>
    <div class="card-header-title">Identity</div>
  </div>
  <div class="card-body">
    <div class="field">
      <label class="field-label">Product name <span class="req">*</span></label>
      <input type="text" class="field-input" name="name" value="{{ .Product.Name }}" required>
    </div>
    <div class="field">
      <label class="field-label">Barcode</label>
      <div style="position:relative;">
        <i class="bi bi-upc-scan" style="position:absolute;left:11px;top:50%;transform:translateY(-50%);color:var(--fg-3);font-size:15px;pointer-events:none;"></i>
        <input type="text" class="field-input mono" name="barcode" value="{{ .Product.Barcode }}" style="padding-left:34px;">
      </div>
    </div>
  </div>
</div>
```

**Card 2 — Categories**

Use tag pill inputs for categories and countries. The hidden inputs store the comma-separated values for Go form parsing. See JS section below for the tag pill logic.

```html
<div class="card">
  <div class="card-header">
    <div class="card-header-icon"><i class="bi bi-grid"></i></div>
    <div class="card-header-title">Categories</div>
  </div>
  <div class="card-body">
    <div class="field">
      <label class="field-label">Product categories</label>
      <input type="hidden" id="categoriesHidden" name="categories" value="{{ .Product.Categories }}">
      <div class="tag-wrap" id="categoriesTags" data-target="categoriesHidden" data-color="accent">
        <!-- JS populates pills from hidden input on load -->
      </div>
      <div class="field-hint">Press Enter to add · × to remove</div>
    </div>
    <div class="field">
      <label class="field-label">Countries sold in</label>
      <input type="hidden" id="countriesHidden" name="countries" value="{{ .Product.Countries }}">
      <div class="tag-wrap" id="countriesTags" data-target="countriesHidden" data-color="neutral">
        <!-- JS populates pills from hidden input on load -->
      </div>
    </div>
  </div>
</div>
```

**Card 3 — Expiry & inventory**
```html
<div class="card">
  <div class="card-header">
    <div class="card-header-icon"><i class="bi bi-calendar-event"></i></div>
    <div class="card-header-title">Expiry & inventory</div>
  </div>
  <div class="card-body">
    <div style="display:grid;grid-template-columns:1fr 1fr;gap:14px;">
      <div class="field">
        <label class="field-label">Expiry date <span class="req">*</span></label>
        <input type="date" class="field-input mono" name="expiry_at"
          value="{{ .Product.ExpiryAt | formatDate }}">
      </div>
      <div class="field">
        <label class="field-label">Amount</label>
        <div class="stepper">
          <button type="button" onclick="let i=this.nextElementSibling;i.value=Math.max(0,+i.value-1)">−</button>
          <input type="number" name="amount" value="{{ .Product.Amount }}" min="0">
          <button type="button" onclick="let i=this.previousElementSibling;i.value=+i.value+1">+</button>
        </div>
      </div>
    </div>
    <div class="field">
      <label class="field-label">Storage location</label>
      <select class="field-input" name="storage_location">
        <option value="">— None —</option>
        {{ range .StorageLocations }}
        <option value="{{ .ID }}" {{ if eq .ID $.Product.StorageLocationID }}selected{{ end }}>
          {{ .Name }}
        </option>
        {{ end }}
      </select>
    </div>
  </div>
</div>
```

**Card 4 — Timestamps (read-only)**
```html
<div class="card">
  <div class="card-header">
    <div class="card-header-icon"><i class="bi bi-clock-history"></i></div>
    <div class="card-header-title">Timestamps</div>
    <span style="font-size:11px;color:var(--fg-3);margin-left:auto;">Read-only</span>
  </div>
  <div class="card-body">
    <div class="meta-grid">
      <div class="meta-item">
        <div class="meta-label">Scanned at</div>
        <div class="meta-value">{{ .Product.ScannedAt | formatDateTime }}</div>
      </div>
      <div class="meta-item">
        <div class="meta-label">Updated at</div>
        <div class="meta-value">{{ .Product.UpdatedAt | formatDateTime }}</div>
      </div>
      <div class="meta-item">
        <div class="meta-label">Notified at</div>
        <div class="meta-value">{{ .Product.NotifiedAt | formatDateTime }}</div>
      </div>
    </div>
  </div>
</div>
```

**3. Form action buttons**

```html
<div class="form-actions">
  <button class="btn-primary" type="submit">
    <i class="bi bi-check2"></i> Update product
  </button>
  <a href="/products/{{ .Product.ID }}" class="btn-secondary">Cancel</a>
  <div class="spacer"></div>
  <button class="btn-danger-ghost" type="button"
    data-bs-toggle="modal" data-bs-target="#deleteModal">
    <i class="bi bi-trash3"></i> Delete
  </button>
</div>
```

**4. Right panel — product image + info**

```html
<div class="product-panel">
  <!-- Image card -->
  <div class="product-image-card">
    <div class="product-image-area">
      {{ if .Product.ImageURL }}
      <img src="{{ .Product.ImageURL }}" alt="{{ .Product.Name }}">
      {{ else }}
      <div style="display:flex;flex-direction:column;align-items:center;justify-content:center;gap:8px;width:100%;height:100%;padding:24px;">
        <i class="bi bi-image" style="font-size:40px;color:var(--fg-3);"></i>
        <span style="font-size:11px;color:var(--fg-3);">No image</span>
      </div>
      {{ end }}
      <div class="product-image-overlay">
        <button type="button" onclick="document.getElementById('fieldImageURL').focus()">
          <i class="bi bi-image"></i> Change image
        </button>
      </div>
    </div>
    <div class="product-info">
      <div class="product-name-display">{{ .Product.Name }}</div>
      <div class="product-barcode-display">
        <i class="bi bi-upc-scan"></i> {{ .Product.Barcode }}
      </div>
      <span class="badge-status badge-{{ .Product.Status }}">
        <i class="bi bi-{{ statusIcon .Product.Status }}"></i>
        {{ .Product.StatusLabel }}
      </span>
    </div>
  </div>

  <!-- Image URL card -->
  <div class="card">
    <div class="card-header">
      <div class="card-header-icon"><i class="bi bi-image"></i></div>
      <div class="card-header-title">Image URL</div>
    </div>
    <div class="card-body" style="padding:14px 16px;">
      <div class="field" style="margin-bottom:0;">
        <input type="url" id="fieldImageURL" class="field-input"
          name="image_url" value="{{ .Product.ImageURL }}"
          placeholder="https://…" style="font-size:12px;font-family:var(--font-mono);">
        <div class="field-hint">Paste a URL or leave blank to remove the image</div>
      </div>
    </div>
  </div>

  <!-- Quick info card -->
  <div class="card">
    <div class="card-header">
      <div class="card-header-icon"><i class="bi bi-info-circle"></i></div>
      <div class="card-header-title">Product info</div>
    </div>
    <div class="card-body" style="padding:14px 16px;display:flex;flex-direction:column;gap:10px;">
      <div style="display:flex;justify-content:space-between;align-items:center;font-size:13px;">
        <span style="color:var(--fg-3);">Status</span>
        <span class="badge-status badge-{{ .Product.Status }}" style="font-size:11px;padding:2px 9px;">
          {{ .Product.StatusLabel }}
        </span>
      </div>
      <div style="border-top:1px solid var(--border-subtle);padding-top:10px;display:flex;justify-content:space-between;align-items:center;font-size:13px;">
        <span style="color:var(--fg-3);">Amount</span>
        <span style="font-weight:600;color:var(--fg);font-family:var(--font-mono);">{{ .Product.Amount }}</span>
      </div>
      <div style="border-top:1px solid var(--border-subtle);padding-top:10px;display:flex;justify-content:space-between;align-items:center;font-size:13px;">
        <span style="color:var(--fg-3);">Expires</span>
        <span style="font-weight:600;font-family:var(--font-mono);">{{ .Product.ExpiryAt | formatDate }}</span>
      </div>
    </div>
  </div>
</div>
```

**5. Delete confirmation modal**

```html
<div class="modal fade" id="deleteModal" tabindex="-1">
  <div class="modal-dialog modal-dialog-centered" style="max-width:400px;">
    <div class="modal-content" style="border-radius:14px;">
      <div class="modal-header d-flex gap-3 align-items-start" style="border-bottom:1px solid var(--border-subtle);padding:18px 20px 14px;">
        <div style="width:38px;height:38px;border-radius:9px;background:oklch(0.95 0.05 25);display:flex;align-items:center;justify-content:center;font-size:18px;color:oklch(0.45 0.20 25);flex-shrink:0;">
          <i class="bi bi-trash3-fill"></i>
        </div>
        <div class="flex-grow-1">
          <h5 class="modal-title" style="font-size:15px;font-weight:700;">Delete {{ .Product.Name }}?</h5>
        </div>
        <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
      </div>
      <div class="modal-body" style="font-size:13px;color:var(--fg-2);line-height:1.6;padding:16px 20px;">
        This product will be permanently deleted. Consider archiving instead if you want to keep the record.
      </div>
      <div class="modal-footer" style="border-top:1px solid var(--border-subtle);padding:12px 20px;">
        <button class="btn-secondary" data-bs-dismiss="modal">Cancel</button>
        <form method="POST" action="/products/{{ .Product.ID }}/delete" style="display:inline;">
          <input type="hidden" name="csrf_token" value="{{ .CSRFToken }}">
          <button class="btn-danger-ghost" type="submit" style="background:oklch(0.55 0.20 25);color:white;border:none;border-radius:8px;padding:10px 18px;">
            <i class="bi bi-trash3"></i> Delete permanently
          </button>
        </form>
      </div>
    </div>
  </div>
</div>
```

**6. Plain JS — tag pill inputs**

Add this to your products JS or a dedicated `tag-input.js` file:

```javascript
// tag-input.js — Proviant tag pill input
// Converts comma-separated hidden input values into interactive pill tags

(function () {
  function initTagInput(wrap) {
    const targetId = wrap.dataset.target;
    const hidden   = document.getElementById(targetId);
    const isAccent = wrap.dataset.color === 'accent';
    if (!hidden) return;

    // Build pills from existing comma-separated value
    const values = hidden.value.split(',').map(v => v.trim()).filter(Boolean);
    values.forEach(v => addTag(wrap, hidden, v, isAccent));

    // Add text input
    const input = document.createElement('input');
    input.className    = 'tag-input';
    input.placeholder  = 'Add…';
    wrap.appendChild(input);

    input.addEventListener('keydown', (e) => {
      if ((e.key === 'Enter' || e.key === ',') && input.value.trim()) {
        e.preventDefault();
        addTag(wrap, hidden, input.value.trim(), isAccent);
        input.value = '';
        syncHidden(wrap, hidden);
      }
      if (e.key === 'Backspace' && !input.value) {
        const tags = wrap.querySelectorAll('.tag, .ctag');
        if (tags.length) tags[tags.length - 1].remove();
        syncHidden(wrap, hidden);
      }
    });

    wrap.addEventListener('click', () => input.focus());
  }

  function addTag(wrap, hidden, text, accent) {
    const tag = document.createElement('span');
    tag.className = accent ? 'tag' : 'ctag';
    tag.innerHTML = `${text} <button type="button"><i class="bi bi-x"></i></button>`;
    tag.querySelector('button').addEventListener('click', (e) => {
      e.stopPropagation();
      tag.remove();
      syncHidden(wrap, hidden);
    });
    // Insert before the input element
    const input = wrap.querySelector('.tag-input');
    wrap.insertBefore(tag, input);
  }

  function syncHidden(wrap, hidden) {
    const tags = [...wrap.querySelectorAll('.tag, .ctag')].map(t => t.textContent.trim());
    hidden.value = tags.join(',');
  }

  // Init all tag wraps on page load
  document.querySelectorAll('.tag-wrap[data-target]').forEach(initTagInput);
})();
```

### CSS to add

Copy all CSS from `design-system/ui_kits/proviant/edit-product.html` between `/* ── App shell ── */` and the end of the `<style>` block. Add after your existing Proviant base CSS.

### Do not change

- Any Go handler for `GET /products/:id/edit` or `POST /products/:id`
- The form `action` and `method` attributes
- CSRF token handling
- Any existing model/database fields

### Verify when done

1. Page shows two columns on desktop (≥768px), single column on mobile
2. Categories and countries render as tag pills from the existing comma-separated values
3. Adding a tag (Enter key) and removing a tag (×) updates the hidden input value
4. Amount stepper increments/decrements correctly, minimum 0
5. Timestamps card shows read-only values with no input fields
6. Product image shows in right panel; "Change image" hover overlay focuses the URL input
7. Delete button opens confirmation modal, not an immediate action
8. Form submits correctly to existing Go handler

---

## Mobile layout

The mobile layout uses three key adaptations. Reference: `design-system/ui_kits/proviant/edit-product-preview.html` — tap the "Mobile" frame to explore.

### Key differences from desktop

| Desktop | Mobile |
|---------|--------|
| Two-column grid | Single column, full width |
| All sections visible | Tabbed: Details / Dates / Info |
| Image in right panel | Compact image strip in topbar |
| Footer inline below form | Sticky fixed footer |
| Delete as ghost text button | Delete as icon-only button |

### 1. Mobile topbar

Replace the page header with a compact topbar with back arrow:

```html
<div class="d-flex align-items-center gap-3 p-3 border-bottom bg-white d-md-none">
  <a href="/products/{{ .Product.ID }}" style="color:var(--fg-2);font-size:20px;">
    <i class="bi bi-arrow-left"></i>
  </a>
  <div class="flex-grow-1">
    <div style="font-size:15px;font-weight:700;color:var(--fg);letter-spacing:-0.01em;">Edit product</div>
    <div style="font-size:10px;color:var(--fg-3);">#{{ .Product.ID }} · {{ .Product.Name }}</div>
  </div>
  <span class="badge-status badge-{{ .Product.Status }}" style="font-size:10px;padding:2px 9px;">
    <i class="bi bi-{{ statusIcon .Product.Status }}"></i> {{ .Product.StatusLabel }}
  </span>
</div>
```

### 2. Image strip (mobile only)

Show a compact image + name strip below the topbar:

```html
<div class="d-flex align-items-center gap-3 p-3 border-bottom bg-white d-md-none">
  <div style="width:52px;height:52px;border-radius:8px;background:var(--bg-subtle);flex-shrink:0;overflow:hidden;display:flex;align-items:center;justify-content:center;">
    {{ if .Product.ImageURL }}
    <img src="{{ .Product.ImageURL }}" alt="{{ .Product.Name }}" style="width:100%;height:100%;object-fit:contain;padding:4px;">
    {{ else }}
    <i class="bi bi-image" style="font-size:22px;color:var(--fg-3);"></i>
    {{ end }}
  </div>
  <div class="flex-grow-1" style="min-width:0;">
    <div style="font-size:13px;font-weight:600;color:var(--fg);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;">{{ .Product.Name }}</div>
    <div style="font-family:var(--font-mono);font-size:10px;color:var(--fg-3);margin-top:2px;">{{ .Product.Barcode }}</div>
  </div>
  <button type="button" class="btn" style="background:var(--accent-subtle);border:1px solid oklch(0.78 0.08 162);border-radius:7px;padding:6px 10px;font-size:11px;color:var(--accent);font-weight:600;"
    onclick="document.getElementById('fieldImageURL').focus(); document.getElementById('tabMeta').click();">
    <i class="bi bi-image"></i> Photo
  </button>
</div>
```

### 3. Tabbed sections (mobile only)

Wrap the form cards in Bootstrap tabs on mobile:

```html
<!-- Tab nav — mobile only -->
<ul class="nav nav-tabs d-md-none border-bottom bg-white" id="editTabs" style="padding:0 4px;">
  <li class="nav-item flex-fill text-center">
    <button class="nav-link active w-100" id="tabDetails" data-bs-toggle="tab" data-bs-target="#panelDetails"
      style="font-size:12px;font-weight:500;border-radius:0;border-bottom:2px solid transparent;">
      Details
    </button>
  </li>
  <li class="nav-item flex-fill text-center">
    <button class="nav-link w-100" id="tabDates" data-bs-toggle="tab" data-bs-target="#panelDates"
      style="font-size:12px;font-weight:500;border-radius:0;border-bottom:2px solid transparent;">
      Dates
    </button>
  </li>
  <li class="nav-item flex-fill text-center">
    <button class="nav-link w-100" id="tabMeta" data-bs-toggle="tab" data-bs-target="#panelMeta"
      style="font-size:12px;font-weight:500;border-radius:0;border-bottom:2px solid transparent;">
      Info
    </button>
  </li>
</ul>

<div class="tab-content d-md-none" style="padding:14px 14px 80px;">
  <!-- Details tab: Identity + Categories -->
  <div class="tab-pane fade show active" id="panelDetails">
    <!-- Card: Identity -->
    <!-- Card: Categories -->
  </div>
  <!-- Dates tab: Expiry & inventory -->
  <div class="tab-pane fade" id="panelDates">
    <!-- Card: Expiry & inventory -->
  </div>
  <!-- Info tab: Timestamps + image URL + quick info -->
  <div class="tab-pane fade" id="panelMeta">
    <!-- Card: Timestamps -->
    <!-- Card: Image URL -->
    <!-- Card: Product info -->
  </div>
</div>

<!-- Desktop: show all cards in normal flow, no tabs -->
<div class="d-none d-md-block">
  <!-- All cards without tab wrappers -->
</div>
```

### 4. Sticky mobile footer

Replace the inline form-actions with a fixed footer on mobile:

```html
<!-- Mobile footer — fixed -->
<div class="d-flex gap-2 d-md-none" style="
  position: fixed; bottom: 0; left: 0; right: 0;
  background: white; border-top: 1px solid var(--border);
  padding: 10px 14px calc(10px + env(safe-area-inset-bottom, 0px));
  box-shadow: 0 -4px 16px oklch(0.18 0.022 70 / 0.08);
  z-index: 100;">
  <button type="submit" form="editProductForm" class="btn btn-proviant-primary flex-grow-1">
    <i class="bi bi-check2"></i> Update product
  </button>
  <a href="/products/{{ .Product.ID }}" class="btn btn-proviant-secondary">Cancel</a>
  <button type="button" class="btn" data-bs-toggle="modal" data-bs-target="#deleteModal"
    style="background:white;border:1px solid var(--border);border-radius:8px;padding:10px 12px;color:var(--status-expired);">
    <i class="bi bi-trash3" style="font-size:16px;"></i>
  </button>
</div>

<!-- Desktop footer — inline (hide on mobile) -->
<div class="form-actions d-none d-md-flex">
  <button type="submit" form="editProductForm" class="btn-primary">
    <i class="bi bi-check2"></i> Update product
  </button>
  <a href="/products/{{ .Product.ID }}" class="btn-secondary">Cancel</a>
  <div class="spacer"></div>
  <button type="button" class="btn-danger-ghost" data-bs-toggle="modal" data-bs-target="#deleteModal">
    <i class="bi bi-trash3"></i> Delete
  </button>
</div>
```

### 5. Hide sidebar on mobile

```css
@media (max-width: 767px) {
  .sidebar { display: none !important; }
  .edit-grid { grid-template-columns: 1fr !important; }
  .product-panel { display: none; } /* shown inline in mobile tabs instead */
}
```

### Mobile verify checklist

1. On mobile (≤767px): sidebar hidden, topbar + image strip visible, tabs present
2. Details tab: Identity and Categories cards only
3. Dates tab: Expiry & inventory card only
4. Info tab: Timestamps, Image URL, Product info cards
5. Sticky footer visible on all tabs with Save / Cancel / Delete icon
6. "Photo" button in image strip switches to Info tab and focuses Image URL field
7. On desktop (≥768px): original two-column layout, no tabs, no mobile topbar
