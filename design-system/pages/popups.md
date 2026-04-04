# Popup Style Guide — Proviant

> **Scope:** All transient overlays, confirmations, and feedback messages in the app.
> **Framework:** Bootstrap 5 (`modal`, `toast`, `alert`) — use Bootstrap primitives; no custom overlay wrappers.

---

## Popup Taxonomy

| Type | Use case | Blocks UI? | Auto-dismiss? |
|------|----------|------------|---------------|
| **Confirmation Modal** | Destructive actions requiring user decision (delete, archive) | Yes | No |
| **Feedback Modal** | Page-level success or error after any async action | Yes | No |
| **Toast** | Transient system feedback during auth flow | No | Yes — 4 s |
| **Inline Alert** | Field-level or section-level validation, right next to the triggering element | No | No |

### When to use Feedback Modal vs Inline Alert

**Use Feedback Modal when:**
- The action trigger (button) may be below the viewport fold
- The message is a response to a primary page action (create product, save settings, accept invite)
- Success or error is not co-located with the triggering element

**Use Inline Alert when:**
- The message belongs directly below a form field (validation error)
- The triggering element and the message are always in the same viewport area
- Auth page login/signup errors (page is short, alerts are directly under buttons)

---

## 1. Feedback Modal (page-level success / error)

The feedback modal is the **default response pattern** for all async page actions. It is always centered, always visible regardless of scroll position.

### Required markup — lives once in `base.tmpl`
```html
<div id="proviantFeedbackModal" class="modal fade" tabindex="-1"
     aria-labelledby="proviantFeedbackModalLabel" aria-hidden="true">
  <div class="modal-dialog modal-dialog-centered modal-sm">
    <div class="modal-content">
      <div class="modal-header border-0 pb-0">
        <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
      </div>
      <div class="modal-body text-center px-4 pb-2">
        <i id="proviantFeedbackIcon" class="bi fs-1 mb-3 d-block" aria-hidden="true"></i>
        <h5 id="proviantFeedbackTitle" class="mb-2"></h5>
        <p id="proviantFeedbackMessage" class="text-body-secondary mb-0 small"></p>
      </div>
      <div class="modal-footer border-0 justify-content-center pt-2 pb-4">
        <button type="button" class="btn btn-primary px-4"
                id="proviantFeedbackBtn" data-bs-dismiss="modal">OK</button>
      </div>
    </div>
  </div>
</div>
```

### Icon + color mapping

| Type | Icon class | Color class |
|------|-----------|-------------|
| `success` | `bi-check-circle-fill` | `text-success` |
| `error` | `bi-x-circle-fill` | `text-danger` |
| `warning` | `bi-exclamation-circle-fill` | `text-warning` |
| `info` | `bi-info-circle-fill` | `text-secondary` |

### JS helper — `proviant.showFeedback(type, title, message, onClose?)`

```javascript
proviant.showFeedback('success', 'Product Created', "Barcode '1234567890123' was added.");
proviant.showFeedback('error',   'Save Failed',     'Request contained invalid data.');
proviant.showFeedback('success', 'Saved', 'Settings updated.', () => location.reload());
```

The optional `onClose` callback fires when the user clicks OK (before the modal hides). Use it for post-action navigation or page reload.

### Rules
- One instance in `base.tmpl` — all pages share it
- `modal-sm` — keeps the dialog compact; no wide body copy needed
- No "Cancel" button — feedback is informational, not a decision point
- Title ≤ 4 words; message ≤ 120 chars
- `textContent` only (no `innerHTML`) — prevents XSS

---

## 2. Confirmation Modal (destructive actions)

### Shared confirm modal — `proviant.showConfirm()`

For inline destructive actions (remove member, cancel invitation, cancel application) use the shared `#proviantConfirmModal` in `base.tmpl` via the JS helper. Reserve page-specific confirmation modals (like delete/archive product) only when the confirmation needs custom button labels or contextual body content that is populated at show-time.

```javascript
// Signature
proviant.showConfirm(title, message, onConfirm, confirmLabel?, confirmType?)

// Examples
proviant.showConfirm('Remove Member', 'Remove this member from the household?', function () { ... });
proviant.showConfirm('Delete Product', 'This cannot be undone.', function () { ... }, 'Delete', 'danger');
```

- `confirmLabel` defaults to `"Confirm"`
- `confirmType` is a Bootstrap button variant; defaults to `"danger"`
- `onConfirm` is called after the modal hides — no event object passed
- Cancel button always present with `data-bs-dismiss="modal"`

### When to use
- Destructive actions (delete, archive, leave household)
- Any action where the user must confirm before proceeding
- Info dialogs that require a button click to dismiss

### Required markup pattern
```html
<div class="modal fade" id="{name}Modal" tabindex="-1"
     aria-labelledby="{name}ModalLabel" aria-hidden="true">
  <div class="modal-dialog modal-dialog-centered">
    <div class="modal-content">
      <div class="modal-header">
        <h5 class="modal-title fs-5" id="{name}ModalLabel">{Title}</h5>
        <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
      </div>
      <div class="modal-body" id="{name}ModalBody">
        {Content}
      </div>
      <div class="modal-footer">
        <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">Cancel</button>
        <button type="button" class="btn btn-{semantic}" id="btn{Name}">
          <i class="bi bi-{icon} me-2"></i>{Action label}
        </button>
      </div>
    </div>
  </div>
</div>
```

### Rules
- `modal fade` — both classes required; `fade` enables Bootstrap's built-in 250ms opacity + vertical slide animation
- `modal-dialog-centered` — always; centers the dialog vertically in the viewport
- `tabindex="-1"` + `aria-labelledby` + `aria-hidden="true"` — required for accessibility
- Close `btn-close` always present in the header
- Backdrop: default Bootstrap backdrop (`data-bs-backdrop` omitted = `true`); clicking outside closes the modal
- Footer always has a **Cancel** button (`data-bs-dismiss="modal"`) as the first/secondary action
- Primary action button uses semantic color: `btn-danger` for delete, `btn-warning` for archive, `btn-primary` for neutral confirm
- Title: `fs-5` — never larger; keep titles short (≤ 5 words)
- No `modal-dialog-centered` override allowed — all modals must center

### Button semantic mapping
| Action | Button class |
|--------|-------------|
| Delete / remove | `btn-danger` |
| Archive | `btn-warning` |
| Confirm / save | `btn-primary` |
| Neutral info | `btn-secondary` |

---

## 3. Toast (auth flow only)

### When to use
- Non-blocking feedback after an async action (product saved, email sent, error on background task)
- Auto-dismissed after 4 seconds; user can close earlier

### Required markup pattern
```html
<!-- Toast container — place once per page, before closing </body> or at top of content -->
<div class="toast-container position-fixed top-0 start-50 translate-middle-x pt-3" style="z-index: 1090;">
  <div id="{name}Toast" class="toast align-items-center text-bg-{semantic} border-0"
       role="alert" aria-live="polite" aria-atomic="true" data-bs-delay="4000">
    <div class="d-flex">
      <div class="toast-body" id="{name}ToastBody">
        {Message}
      </div>
      <button type="button" class="btn-close btn-close-white me-2 m-auto"
              data-bs-dismiss="toast" aria-label="Close"></button>
    </div>
  </div>
</div>
```

### Rules
- Container: `position-fixed top-0 start-50 translate-middle-x pt-3` — centered horizontally at top
- Container `z-index: 1090` — above modals (Bootstrap modals use 1055/1060), below tooltips (1080... wait, Bootstrap tooltips 1080 < 1090; actually Bootstrap z-index: modal=1055, modal-backdrop=1050, toast=1090 by default) — use `z-index: 1090`
- `data-bs-delay="4000"` — 4 seconds auto-dismiss (3–5 s per UX rule)
- Semantic color: use Bootstrap's `text-bg-{variant}` composite utility — `text-bg-success`, `text-bg-danger`, `text-bg-warning`, `text-bg-info` — never raw hex on toast background
- Close button: `btn-close btn-close-white` (white × on colored background)
- `aria-live="polite"` for success/info; `aria-live="assertive"` for errors
- `aria-atomic="true"` always
- Toast body text stays under ~80 characters; no multi-line body
- One toast visible at a time per page; new toast replaces old (hide previous before showing new)

### JS show pattern
```javascript
const toast = bootstrap.Toast.getOrCreateInstance(document.getElementById('{name}Toast'));
document.getElementById('{name}ToastBody').textContent = message;
toast.show();
```

---

## 4. Inline Alert

### When to use
- Form validation errors or success below a form section
- Contextual feedback that belongs in the document flow (not floating/overlay)

### Required markup pattern
```html
<div id="{section}Alert" class="alert alert-{semantic} fade d-none" role="alert">
  <i class="bi bi-{icon} me-2"></i><span id="{section}AlertMessage"></span>
</div>
```

### Rules
- Initial state: `d-none` (hidden) — **never** `style="display: none"`
- `fade` class on the element — Bootstrap transitions `opacity` when `show` is added/removed
- Semantic class set at show-time by JS (not hardcoded unless always the same intent)
- **No** `alert-dismissible` + close button unless the alert is persistent / not auto-managed by JS
- Show via JS: `el.classList.remove('d-none'); el.classList.add('show');`
- Hide via JS: `el.classList.remove('show'); el.classList.add('d-none');`
- Message always in a child `<span>` (or set `textContent`); never `innerHTML` with user content (XSS)

### Semantic class mapping (set dynamically)
```javascript
function showAlert(el, isSuccess, message) {
  el.className = `alert fade ${isSuccess ? 'alert-success' : 'alert-danger'}`;
  el.querySelector('span').textContent = message;
  el.classList.remove('d-none');
  el.classList.add('show');
}
function hideAlert(el) {
  el.classList.remove('show');
  el.classList.add('d-none');
}
```

---

## 5. Animation Summary

All animation uses Bootstrap's built-in CSS transitions — no custom JS animation needed.

| Component | Mechanism | Duration |
|-----------|-----------|----------|
| Modal open | `.modal.fade` → `.modal.fade.show` → CSS `opacity` + `transform: translateY()` | ~300ms (Bootstrap default) |
| Modal close | Reverse | ~300ms |
| Toast show | `.toast.fade` → `.toast.fade.show` → CSS `opacity` | ~150ms (Bootstrap default) |
| Inline alert | `.alert.fade` → `.alert.fade.show` → CSS `opacity` | ~150ms |

All transitions respect `@media (prefers-reduced-motion: reduce)` — already handled in `main.scss`.

---

## 6. Anti-Patterns

- ❌ `modal` without `fade` — snaps in/out with no animation
- ❌ `modal-dialog` without `modal-dialog-centered` — appears at top of viewport
- ❌ `style="display: none"` for alert toggle — use `d-none` class
- ❌ `data-bs-backdrop="static"` on non-form modals — always allow click-outside dismiss
- ❌ Raw hex on toast background (`bg-success text-dark` hardcoded) — use `text-bg-success`
- ❌ Toast delay > 5000ms or < 3000ms — out of UX range
- ❌ `<strong class="me-auto">proviant</strong>` toast header — toasts use the compact single-row pattern (no header section needed)
- ❌ Generic modal title like "proviant" — title must describe the action
- ❌ `innerHTML` with user-controlled content in alerts (XSS risk) — use `textContent`

---

## 7. Checklist (pre-delivery)

- [ ] `#proviantFeedbackModal` present once in `base.tmpl`
- [ ] All page-level success/error responses call `proviant.showFeedback()` — no new inline alerts for async results
- [ ] Modal has `fade` + `modal-dialog-centered`
- [ ] Modal has close button in header + cancel in footer
- [ ] Toast container is `position-fixed top-0 start-50 translate-middle-x`
- [ ] Toast uses `text-bg-{semantic}` (not raw hex)
- [ ] Toast `data-bs-delay="4000"`
- [ ] Inline alerts start with `d-none`, not `style="display:none"`
- [ ] Inline alerts have `fade` class for smooth transition
- [ ] Alert message uses `textContent` (not `innerHTML`) for user-provided text
