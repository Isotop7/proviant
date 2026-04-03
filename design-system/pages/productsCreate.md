# Page Override: Create Product (`/web/products/create`)

> Overrides apply on top of `design-system/MASTER.md`. Only differences from Master are listed.

---

## Priority Focus

Scan-first data entry. The primary action is scanning a barcode — everything else is secondary. Progressive disclosure keeps the UI uncluttered until a barcode is resolved.

---

## Layout

- Page content constrained to `col-12 col-md-8 col-lg-6` — narrower than the default container, optimised for single-column form flow on all screen sizes.

---

## Progressive Disclosure — 2 Zones

Zone 2 (`#productData`) starts hidden (`d-none`) and is revealed by JS once a barcode is committed via scan or manual input. Users are never shown empty form fields before they have a barcode.

| Zone | Visibility | Content |
|------|-----------|---------|
| Zone 1 | Always visible | Heading, hero scan button, camera view, manual barcode input |
| Zone 2 | Revealed on barcode | Product image + name, instance dropdown, expiry date, action buttons |

---

## Scan Button

Full-width hero button (`d-grid btn-lg py-3`) — scan is the primary action, not a secondary input beside a text field. Icon uses `fs-3` for visual emphasis.

---

## Button Hierarchy

| Tier | Elements | Style |
|------|----------|-------|
| Primary | Scan barcode, Save product | `btn-primary btn-lg` |
| Secondary | View, All instances | `btn-sm btn-outline-secondary` |
| Destructive | Archive, Delete | `btn-sm btn-outline-warning/danger` |
| Tertiary | Quick-pick (+3d, +7d, +1m) | `btn-sm btn-outline-secondary` |

---

## Confirmations

Archive and Delete do **not** use page-specific modals. Both trigger `proviant.showConfirm()` via JS, which uses the shared `#proviantConfirmModal` from `base.tmpl`.

---

## Feedback

All async results (product created, errors) use `proviant.showFeedback()` — no inline alert on this page.
