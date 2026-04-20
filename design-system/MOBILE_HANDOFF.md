# Proviant Mobile Scan-First — Claude Code Implementation Prompt

Paste this prompt directly into Claude Code in your project root.

---

## Prompt

I'm implementing a mobile-first redesign of the Proviant add-product flow. Read the design reference file at `design-system/ui_kits/proviant/mobile-scan-home.html` — it's a fully working Bootstrap 5 + plain JS prototype showing the exact layout, CSS classes, and interaction flow to implement. Also read `design-system/colors_and_type.css` for all CSS tokens.

### What to implement

**1. New bottom navigation bar**

Replace the existing sidebar with a bottom nav bar on mobile (≤768px). The bottom nav has 5 items in this order: Dashboard · Products · Scan · Archive · Settings. "Scan" is the center tab and is visually prominent (larger icon, teal color when active). Copy the `.bottom-nav` and `.bottom-nav-item` CSS from the reference file.

```html
<!-- Go template: base layout bottom nav -->
<nav class="bottom-nav">
  <a href="/dashboard" class="bottom-nav-item {{ if eq .Page "dashboard" }}active{{ end }}">
    <i class="bi bi-speedometer2"></i><span>Dashboard</span>
  </a>
  <a href="/products" class="bottom-nav-item {{ if eq .Page "products" }}active{{ end }}">
    <i class="bi bi-box-seam"></i><span>Products</span>
  </a>
  <a href="/scan" class="bottom-nav-item nav-scan {{ if eq .Page "scan" }}active{{ end }}">
    <i class="bi bi-upc-scan"></i><span>Scan</span>
  </a>
  <a href="/archive" class="bottom-nav-item {{ if eq .Page "archive" }}active{{ end }}">
    <i class="bi bi-archive"></i><span>Archive</span>
  </a>
  <a href="/settings" class="bottom-nav-item {{ if eq .Page "settings" }}active{{ end }}">
    <i class="bi bi-person-circle"></i><span>Settings</span>
  </a>
</nav>
```

The bottom nav should only show on mobile. On desktop (≥768px), keep the existing sidebar. Use a media query:

```css
@media (min-width: 768px) { .bottom-nav { display: none; } }
@media (max-width: 767px) { .pv-sidebar { display: none; } .app-content { padding-bottom: 70px; } }
```

**2. Redesign the /scan (add product) page**

The scan page is the primary mobile home screen. Replace the current layout with three sections:

- **Header:** page title "Add a product" + subtitle "Scan any EAN barcode to get started"
- **Viewfinder area:** a dark rounded container (`.viewfinder`) that shows the camera feed. Add corner bracket marks (`.viewfinder-corner`) using CSS borders. When scanning is active, show an animated `.scan-line` element.
- **Action buttons:** a large teal `.btn-scan-primary` button that activates the camera, a divider "— or —", and a `.btn-scan-manual` secondary button that opens a Bootstrap modal for manual barcode entry.

Copy the exact CSS for `.viewfinder`, `.viewfinder-corner`, `.scan-line`, `.btn-scan-primary`, `.btn-scan-manual` from `design-system/ui_kits/proviant/mobile-scan-home.html`.

**3. Pre-filled form after barcode scan**

After a successful barcode lookup (your existing Open Food Facts integration), render a confirmation form with:

- A `.scan-result` success banner showing the matched product name and barcode
- Pre-filled inputs for: Product name, Category (select), Expiry date (3 separate DD/MM/YYYY inputs with `font-family: var(--font-mono)`)
- A full-width `.btn-add` submit button

```html
<!-- Go template: scan result banner -->
{{ if .ScannedProduct }}
<div class="scan-result mb-3">
  <i class="bi bi-check-circle-fill"></i>
  <div>
    <div class="scan-result-name">{{ .ScannedProduct.Name }}</div>
    <div class="scan-result-code">{{ .ScannedProduct.Barcode }} · Open Food Facts</div>
  </div>
</div>
{{ end }}
```

**4. Success state**

After form submission, show a centered success screen with:
- A circular teal icon container with `bi-check-circle-fill`
- Product name confirmation
- Status badge (`badge-status badge-{{ .Product.Status }}`)
- Two buttons: "Scan another" (links to /scan) and "View products" (links to /products)

**5. Dashboard hero scan card**

On the dashboard page (`/dashboard`), add a hero card above the existing content:

```html
<a href="/scan" class="hero-scan-card">
  <div style="position:relative;">
    <div class="hero-label">Quick action</div>
    <div class="hero-title">Scan a product</div>
    <div class="hero-sub">Add to your pantry in seconds</div>
    <div class="hero-pill">
      <i class="bi bi-upc-scan"></i> Start scanning
    </div>
  </div>
</a>
```

Copy `.hero-scan-card`, `.hero-label`, `.hero-title`, `.hero-sub`, `.hero-pill` CSS from the reference file.

### CSS to add to your main stylesheet

Copy all CSS from `design-system/ui_kits/proviant/mobile-scan-home.html` between the `/* ── Proviant mobile base ── */` and the end of the `<style>` block. Add it to your existing stylesheet after the Proviant base tokens from `design-system/colors_and_type.css`.

### Do not change

- Any Go handler or controller files
- The barcode scanning JavaScript (camera API integration)
- The Open Food Facts API call logic
- Any existing database or model code
- Desktop layout (sidebar, header) — only add mobile overrides

### Files to read before starting

```
design-system/mobile-dashboard.html   ← Full reference implementation
design-system/colors_and_type.css                      ← CSS tokens
design-system/HANDOFF.md                               ← Full refactor guide
```

### Verify when done

1. On a mobile viewport (390px wide), the sidebar is hidden and bottom nav is visible
2. The Scan tab is the center item and shows a larger icon
3. `/scan` shows the viewfinder container + scan button + manual entry option
4. After scan lookup, the pre-filled form renders with the `.scan-result` banner
5. The dashboard has the hero scan card above the existing content
6. On desktop (≥768px), the layout is unchanged

---

## Desktop Prompt — Modal dialog for add product

Paste this second prompt into Claude Code after the mobile implementation is done.

---

I'm adding a desktop add-product modal to Proviant. Read `design-system/preview/desktop-scan-layouts.html` — variant 1 (Modal dialog) is the approved pattern. Also read `design-system/colors_and_type.css` for tokens.

### What to implement

**1. "Add product" button in the products page header**

On desktop (≥768px), add a button to the right of the products page heading:

```html
<!-- templates/products.html -->
<div class="d-flex align-items-center justify-content-between mb-4">
  <div>
    <h1 style="font-size:18px; font-weight:700; letter-spacing:-0.02em;">Products</h1>
    <p style="font-size:12px; color:var(--fg-3);">{{ .ProductCount }} items in your household</p>
  </div>
  <button class="btn btn-proviant-primary" id="btnAddProduct">
    <i class="bi bi-plus-lg"></i> Add product
  </button>
</div>
```

**2. Modal HTML**

Add this modal to the bottom of `templates/products.html` (or your base layout):

```html
<div class="modal fade" id="addProductModal" tabindex="-1" aria-labelledby="addProductModalLabel">
  <div class="modal-dialog modal-dialog-centered" style="max-width:480px;">
    <div class="modal-content" style="border-radius:14px; border:1px solid var(--border); box-shadow: var(--shadow-xl);">

      <!-- Modal header -->
      <div class="modal-header" style="padding:18px 20px 14px; border-bottom:1px solid var(--border-subtle); gap:12px; align-items:flex-start;">
        <div style="width:38px; height:38px; border-radius:9px; background:var(--accent-subtle); display:flex; align-items:center; justify-content:center; font-size:18px; color:var(--accent); flex-shrink:0;">
          <i class="bi bi-upc-scan"></i>
        </div>
        <div style="flex:1;">
          <div style="font-size:15px; font-weight:700; color:var(--fg);" id="addProductModalLabel">Add product</div>
          <div style="font-size:12px; color:var(--fg-3);" id="addProductModalSubtitle">Scan barcode or enter manually</div>
        </div>
        <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
      </div>

      <!-- Step 1: Barcode scan -->
      <div id="stepScan" class="modal-body" style="padding:18px 20px;">
        <div style="font-size:12px; font-weight:600; color:var(--fg-3); text-transform:uppercase; letter-spacing:0.06em; margin-bottom:8px;">Barcode</div>
        <div id="barcodeInputWrap" style="position:relative; background:white; border:2px solid var(--accent); border-radius:10px; padding:14px 16px; display:flex; align-items:center; gap:10px; box-shadow:0 0 0 4px oklch(0.50 0.12 162 / 0.12); cursor:text;" onclick="document.getElementById('barcodeInput').focus()">
          <i class="bi bi-upc-scan" style="font-size:22px; color:var(--accent); flex-shrink:0;"></i>
          <input id="barcodeInput" type="text" placeholder="Scan or type EAN barcode…"
            style="border:none; outline:none; font-family:var(--font-mono); font-size:16px; color:var(--fg); background:transparent; width:100%; letter-spacing:0.04em;"
            autocomplete="off" autocorrect="off" spellcheck="false">
          <span style="font-family:var(--font-mono); font-size:11px; color:var(--fg-3); flex-shrink:0; background:var(--bg-subtle); border-radius:5px; padding:3px 7px;">↵ Enter</span>
        </div>
        <div style="font-size:12px; color:var(--fg-3); margin-top:8px; display:flex; align-items:center; gap:6px;">
          <i class="bi bi-keyboard"></i> USB/BT scanner — just start scanning, input auto-captures
        </div>
      </div>

      <!-- Step 2: Pre-filled form (hidden until scan) -->
      <div id="stepForm" class="modal-body" style="padding:18px 20px; display:none;">
        <!-- Scan result banner -->
        <div id="scanResultBanner" style="background:var(--accent-subtle); border:1px solid oklch(0.78 0.08 162); border-radius:10px; padding:12px 14px; display:flex; align-items:center; gap:10px; margin-bottom:16px;">
          <i class="bi bi-check-circle-fill" style="color:var(--accent); font-size:16px; flex-shrink:0;"></i>
          <div>
            <div id="scanResultName" style="font-size:13px; font-weight:600; color:var(--fg);"></div>
            <div id="scanResultCode" style="font-size:11px; color:var(--fg-3); font-family:var(--font-mono);"></div>
          </div>
        </div>
        <form id="addProductForm" method="POST" action="/products/add">
          <input type="hidden" name="csrf_token" value="{{ .CSRFToken }}">
          <input type="hidden" id="hiddenBarcode" name="barcode">
          <div class="mb-3">
            <label class="form-label" style="font-size:12px; font-weight:500;">Product name <span style="color:var(--status-expired)">*</span></label>
            <input type="text" id="fieldName" name="name" class="form-control" required>
          </div>
          <div class="row g-2 mb-3">
            <div class="col">
              <label class="form-label" style="font-size:12px; font-weight:500;">Category</label>
              <select id="fieldCategory" name="category" class="form-control">
                <option>Dairy</option><option>Produce</option><option>Beverages</option>
                <option>Bakery</option><option>Meat</option><option>Frozen</option>
                <option>Pantry</option><option>Other</option>
              </select>
            </div>
            <div class="col-auto">
              <label class="form-label" style="font-size:12px; font-weight:500;">Barcode</label>
              <input type="text" id="fieldBarcodeDisplay" class="form-control" readonly
                style="font-family:var(--font-mono); font-size:13px; background:var(--bg-subtle); color:var(--fg-2); width:150px;">
            </div>
          </div>
          <div class="mb-3">
            <label class="form-label" style="font-size:12px; font-weight:500;">Expiry date</label>
            <div class="d-flex gap-2">
              <input type="text" name="expiry_day"   placeholder="DD"   maxlength="2" class="form-control text-center" style="font-family:var(--font-mono); width:64px; flex:none;">
              <input type="text" name="expiry_month" placeholder="MM"   maxlength="2" class="form-control text-center" style="font-family:var(--font-mono); width:64px; flex:none;">
              <input type="text" name="expiry_year"  placeholder="YYYY" maxlength="4" class="form-control text-center" style="font-family:var(--font-mono); width:88px; flex:none;">
            </div>
            <div style="font-size:11px; color:var(--fg-3); margin-top:4px;">Leave blank if unknown</div>
          </div>
        </form>
      </div>

      <!-- Step 3: Success (hidden until submit) -->
      <div id="stepSuccess" class="modal-body" style="padding:18px 20px; display:none;">
        <div style="background:var(--accent-subtle); border:1px solid oklch(0.78 0.08 162); border-radius:12px; padding:16px 18px; display:flex; align-items:center; gap:14px;">
          <div style="width:40px; height:40px; border-radius:999px; background:oklch(0.40 0.10 162); display:flex; align-items:center; justify-content:center; flex-shrink:0;">
            <i class="bi bi-check-lg" style="color:white; font-size:20px;"></i>
          </div>
          <div style="flex:1;">
            <div id="successName" style="font-size:14px; font-weight:600; color:var(--fg);"></div>
            <div style="font-size:12px; color:var(--fg-3);">Successfully added to your pantry</div>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <div class="modal-footer" style="padding:12px 20px; border-top:1px solid var(--border-subtle);">
        <div id="footerScan" style="display:flex; gap:8px; width:100%; justify-content:flex-end;">
          <button type="button" class="btn btn-proviant-secondary" data-bs-dismiss="modal">Cancel</button>
        </div>
        <div id="footerForm" style="display:none; gap:8px; width:100%; justify-content:flex-end;">
          <button type="button" class="btn btn-proviant-secondary" id="btnBackToScan">
            <i class="bi bi-arrow-left"></i> Back
          </button>
          <button type="submit" form="addProductForm" class="btn btn-proviant-primary" id="btnSubmitForm">
            <i class="bi bi-plus-lg"></i> Add to pantry
          </button>
        </div>
        <div id="footerSuccess" style="display:none; gap:8px; width:100%; justify-content:flex-end;">
          <button type="button" class="btn btn-proviant-secondary" id="btnScanAnother">
            <i class="bi bi-upc-scan"></i> Scan another
          </button>
          <button type="button" class="btn btn-proviant-secondary" data-bs-dismiss="modal">Close</button>
        </div>
      </div>

    </div>
  </div>
</div>
```

**3. Plain JS — auto-focus, scanner detection, step transitions**

Add this script to your products page (or a dedicated `scan-modal.js` file):

```javascript
// scan-modal.js — Proviant add product modal
// Works with Bootstrap 5 modals + Go template form submission

(function () {
  const SCANNER_SPEED_MS = 100; // chars arriving faster than this = scanner input
  const MIN_BARCODE_LEN  = 8;

  let lastInputTime = 0;
  let inputBuffer   = '';
  let scannerTimer  = null;

  const modal        = document.getElementById('addProductModal');
  const bsModal      = new bootstrap.Modal(modal);
  const barcodeInput = document.getElementById('barcodeInput');

  // Steps
  const steps    = { scan: 'stepScan',    form: 'stepForm',    success: 'stepSuccess'    };
  const footers  = { scan: 'footerScan',  form: 'footerForm',  success: 'footerSuccess'  };

  function showStep(step) {
    Object.values(steps).forEach(id => document.getElementById(id).style.display = 'none');
    Object.values(footers).forEach(id => { document.getElementById(id).style.display = 'none'; });
    document.getElementById(steps[step]).style.display   = 'block';
    document.getElementById(footers[step]).style.display = 'flex';
  }

  // Auto-focus barcode input when modal opens
  modal.addEventListener('shown.bs.modal', () => {
    showStep('scan');
    barcodeInput.value = '';
    barcodeInput.focus();
  });

  // Scanner speed detection: auto-submit if ≥ MIN_BARCODE_LEN chars in SCANNER_SPEED_MS
  barcodeInput.addEventListener('input', () => {
    const now = Date.now();
    if (now - lastInputTime < SCANNER_SPEED_MS) {
      inputBuffer = barcodeInput.value;
      clearTimeout(scannerTimer);
      scannerTimer = setTimeout(() => {
        if (inputBuffer.length >= MIN_BARCODE_LEN) lookupBarcode(inputBuffer);
      }, SCANNER_SPEED_MS);
    }
    lastInputTime = now;
  });

  // Manual Enter key
  barcodeInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && barcodeInput.value.length >= MIN_BARCODE_LEN) {
      e.preventDefault();
      lookupBarcode(barcodeInput.value);
    }
  });

  // Barcode lookup (calls your existing Go backend)
  async function lookupBarcode(barcode) {
    try {
      const res  = await fetch(`/api/barcode/${encodeURIComponent(barcode)}`);
      const data = await res.json();
      prefillForm(barcode, data.name || '', data.category || '');
    } catch (err) {
      // If lookup fails, still show form with barcode pre-filled
      prefillForm(barcode, '', '');
    }
  }

  function prefillForm(barcode, name, category) {
    document.getElementById('hiddenBarcode').value        = barcode;
    document.getElementById('fieldBarcodeDisplay').value  = barcode;
    document.getElementById('fieldName').value            = name;
    document.getElementById('scanResultName').textContent = name || 'Unknown product';
    document.getElementById('scanResultCode').textContent = barcode + ' · Open Food Facts';
    if (category) {
      const sel = document.getElementById('fieldCategory');
      [...sel.options].forEach(o => { o.selected = o.text === category; });
    }
    showStep('form');
    document.getElementById('addProductModalSubtitle').textContent = 'Confirm details';
    document.getElementById('fieldName').focus();
  }

  // Back button
  document.getElementById('btnBackToScan').addEventListener('click', () => {
    showStep('scan');
    barcodeInput.value = '';
    barcodeInput.focus();
    document.getElementById('addProductModalSubtitle').textContent = 'Scan barcode or enter manually';
  });

  // Form submit → success step (actual submit handled by Go)
  // If using AJAX submit, replace with fetch + show success:
  document.getElementById('addProductForm').addEventListener('submit', (e) => {
    // Remove this block if using standard form POST (Go will redirect)
    // Keep only for AJAX-style submit:
    // e.preventDefault();
    // ... fetch POST, then:
    // document.getElementById('successName').textContent = document.getElementById('fieldName').value;
    // showStep('success');
  });

  // Scan another — reset to scan step
  document.getElementById('btnScanAnother').addEventListener('click', () => {
    showStep('scan');
    barcodeInput.value = '';
    barcodeInput.focus();
    document.getElementById('addProductModalSubtitle').textContent = 'Scan barcode or enter manually';
  });

  // Trigger modal from header button
  document.getElementById('btnAddProduct').addEventListener('click', () => bsModal.show());

})();
```

### CSS to add

Copy the modal-specific CSS from `design-system/ui_kits/proviant/bootstrap-snippets.html` (`.modal-content`, `.modal-header`, `.modal-footer` overrides). These are already in the snippets file.

### Do not change

- Your existing barcode scanning camera logic (mobile)
- Any Go handler for `/products/add` or `/api/barcode/:code`
- The sidebar or desktop layout

### Verify when done

1. Clicking "Add product" opens the modal with the barcode input auto-focused
2. Triggering a USB/BT scanner fills the input and auto-advances to the form (no button press)
3. Pressing Enter manually also advances to the form
4. The pre-filled form shows the scan result banner with product name + barcode
5. Submitting the form posts to `/products/add`
6. On mobile (≤768px) the modal does not appear — users use the scan tab instead
