# Page Design Guide: Create / Scan Product (`/web/products/create`)

> This file is an implementation guide. When implementing, use the exact HTML below as the new body of `productsCreate.tmpl`. All existing JS element IDs are preserved — no JavaScript changes are required.

---

## Design Goals

1. **Scan is the primary action** — not an afterthought beside a text input
2. **Progressive disclosure** — show only what the user needs at each step
3. **No visual overload** — nothing is visible until it's relevant

---

## Layout: 2 zones

```
┌─────────────────────────────────────┐
│  Zone 1 — Always visible            │
│  • Page heading                     │
│  • [SCAN BARCODE] hero button       │
│  • Camera view (hidden until scan)  │
│  • "Or enter manually" + input      │
└─────────────────────────────────────┘
        ↓ after barcode resolved
┌─────────────────────────────────────┐
│  Zone 2 — d-none → shown by JS     │
│  • Product image + name (row)       │
│  • Instance dropdown (if dupe)      │
│  • Expiry date (large input)        │
│  • Quick-pick buttons               │
│  • [SAVE PRODUCT] primary CTA       │
│  • View / All instances (secondary) │
│  • Archive / Delete (destructive)   │
└─────────────────────────────────────┘
```

> **Note:** Archive and Delete confirmations use the shared `#proviantConfirmModal` in `base.tmpl` via `proviant.showConfirm()`. No page-specific modals are needed on this page.

---

## Complete Template

```html
{{ define "content" }}
<div class="row justify-content-center p-3">
  <div class="col-12 col-md-8 col-lg-6">

    <!-- ═══════════════════════════════════════════ -->
    <!-- ZONE 1 — Always visible                     -->
    <!-- ═══════════════════════════════════════════ -->

    <!-- Heading -->
    <div class="text-center mb-4">
      <h2 class="mb-1">Add product</h2>
      <p class="text-muted small mb-0">Scan or enter a barcode</p>
    </div>

    <!-- Alert (hidden until JS triggers it) -->
    <div id="productAlert" class="alert alert-dismissible fade mb-3" role="alert" aria-live="polite">
      <span id="alertMessage"></span>
      <button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>
    </div>

    <!-- Hero scan button -->
    <div class="d-grid mb-3">
      <button class="btn btn-primary btn-lg py-3" type="button" id="btnScan" data-action="scan">
        <i class="bi bi-upc-scan fs-3 me-2"></i>
        <span>Scan barcode</span>
      </button>
    </div>

    <!-- Camera view (hidden until scan active) -->
    <div class="mb-3" id="barcode-reader-wrapper" style="display:none;">
      <div id="barcode-reader"></div>
    </div>

    <!-- Manual barcode entry (secondary) -->
    <form id="formBarcodeInput" class="needs-validation mb-4" novalidate>
      <label for="barcode" class="form-label text-muted small">Or enter barcode manually</label>
      <div class="input-group">
        <span class="input-group-text"><i class="bi bi-upc"></i></span>
        <input type="number" class="form-control" id="barcode"
               placeholder="13-digit EAN barcode"
               aria-label="Barcode" data-barcode=""
               min="1000000000000" max="9999999999999">
        <div class="invalid-feedback">Must be a 13-digit number</div>
        <div class="valid-feedback">Looks valid — fetching product info…</div>
      </div>
    </form>

    <!-- ═══════════════════════════════════════════ -->
    <!-- ZONE 2 — Hidden on load, shown by JS       -->
    <!-- ═══════════════════════════════════════════ -->

    <div id="productData" class="d-none">
      <div class="card mb-3">
        <div class="card-body">

          <!-- Product image + name — compact side-by-side row -->
          <div class="d-flex align-items-center gap-3 mb-3">
            <div class="flex-shrink-0">
              <img id="productInfoImage" src="" alt=""
                   class="rounded" style="width:64px;height:64px;object-fit:contain;background:#EBF5EB;">
              <div class="fs-2 placeholder text-muted d-none">
                <i class="bi bi-image"></i>
              </div>
              <div class="spinner-grow spinner-grow-sm text-secondary" role="status" style="display:none;">
                <span class="visually-hidden">Loading…</span>
              </div>
            </div>
            <div class="flex-grow-1">
              <div id="productInfoName" class="fw-semibold"></div>
              <div id="productInfoGenericName" class="text-muted small"></div>
              <div class="spinner-grow spinner-grow-sm text-secondary" role="status" style="display:none;">
                <span class="visually-hidden">Loading…</span>
              </div>
            </div>
          </div>

          <!-- Instance dropdown (populated + shown by JS when duplicates exist) -->
          <select class="form-select mb-3 d-none" id="instanceDropdown" aria-label="Select existing instance"></select>

          <!-- Expiry date — prominent, large tap target -->
          <form id="formExpireAt" class="needs-validation" novalidate>
            <label for="expireAt" class="form-label fw-medium">
              Expiry date <span class="text-danger" aria-hidden="true">*</span>
            </label>
            <input type="date" class="form-control form-control-lg mb-2" id="expireAt"
                   value="{{ today }}" required aria-required="true">
            <div class="invalid-feedback">Please select an expiry date</div>
          </form>

          <!-- Quick-pick — compact, secondary visual weight -->
          <div class="btn-group w-100" role="group" aria-label="Quick expiry shortcuts">
            <button type="button" class="btn btn-outline-secondary btn-sm" id="btnExpireAdd3">
              <i class="bi bi-calendar-plus me-1"></i>+3 days
            </button>
            <button type="button" class="btn btn-outline-secondary btn-sm" id="btnExpireAdd7">
              <i class="bi bi-calendar-plus me-1"></i>+7 days
            </button>
            <button type="button" class="btn btn-outline-secondary btn-sm" id="btnExpireAdd1m">
              <i class="bi bi-calendar-plus me-1"></i>+1 month
            </button>
          </div>

        </div>
      </div>

      <!-- Primary CTA — full-width, visually dominant -->
      <div class="d-grid mb-2">
        <button id="btnAddProduct" type="submit" class="btn btn-primary btn-lg">
          <i class="bi bi-plus-circle me-2"></i>Save product
        </button>
      </div>

      <!-- Secondary navigation actions -->
      <div class="d-flex gap-2 mb-2" id="foundBarcode">
        <button id="btnShowProduct" type="button" class="btn btn-outline-secondary flex-fill" disabled>
          <i class="bi bi-eye me-1"></i>View
        </button>
        <button id="btnShowProducts" type="button" class="btn btn-outline-secondary flex-fill" disabled>
          <i class="bi bi-collection me-1"></i>All instances
        </button>
      </div>

      <!-- Destructive actions — outline only, visually separated -->
      <!-- Clicking these triggers proviant.showConfirm() in productsCreate.js — no data-bs-toggle needed -->
      <div class="d-flex gap-2">
        <button id="btnArchiveProductModal" type="button" class="btn btn-outline-warning flex-fill" disabled>
          <i class="bi bi-archive me-1"></i>Archive
        </button>
        <button id="btnDeleteProductModal" type="button" class="btn btn-outline-danger flex-fill" disabled>
          <i class="bi bi-trash me-1"></i>Delete
        </button>
      </div>
    </div>

  </div>
</div>

<script src="/assets/js/html5-qrcode.min.js"></script>
<script src="/assets/js/productsCreate.js"></script>
{{ end }}
```

---

## What changed vs original template

| Area | Before | After |
|------|--------|-------|
| Scan button | `col-3`, small | `d-grid btn-lg py-3`, full-width hero |
| Barcode input label | None (placeholder only) | Visible label "Or enter barcode manually" |
| Product info card | Always visible (empty state) | `d-none` on load, revealed by JS |
| Image + name | 3 stacked `list-group-item` rows | 1 compact `d-flex` row |
| Expiry input | `form-control` | `form-control-lg` (larger tap target) |
| Quick-pick style | `btn-outline-info` | `btn-outline-secondary btn-sm` (quieter) |
| Primary CTA | One of 5 buttons in a toolbar | `d-grid btn-lg` — only dominant button |
| Secondary/destructive | Mixed in same toolbar | Separated into 2 `d-flex gap-2` rows |
| Page max-width | Full container | `col-md-8 col-lg-6` — comfortable reading width |
| Archive/Delete modals | Page-specific `#archiveModal` / `#deleteModal` | Removed — use shared `proviant.showConfirm()` |
| Archive/Delete buttons | `data-bs-toggle="modal"` triggers | Plain buttons; JS calls `proviant.showConfirm()` |

## JS element IDs

`barcode`, `barcode-reader`, `barcode-reader-wrapper`, `btnScan`, `btnAddProduct`, `btnShowProduct`, `btnArchiveProductModal`, `btnDeleteProductModal`, `btnShowProducts`, `btnExpireAdd3`, `btnExpireAdd7`, `btnExpireAdd1m`, `productData`, `foundBarcode`, `productInfoImage`, `productInfoName`, `productInfoGenericName`, `expireAt`, `instanceDropdown`

## No other files need changes
