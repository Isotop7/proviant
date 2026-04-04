# Page Override: Products List (`/web/products`)

> Overrides apply on top of `design-system/MASTER.md`. Only differences from Master are listed.

---

## Priority Focus

Core screen — users spend most time here. Information density and expiry readability are the top concerns.

---

## Layout Override

- Grid: `row-cols-1 row-cols-sm-1 row-cols-md-2 row-cols-lg-3` (Master default lg-3 only — add md-2)
- Card gap: `g-4` (not `g-5` — slightly tighter for density)

---

## Status Badge — Expiry Countdown

For ≤ 7 days remaining, prefer a human-readable countdown ("3 days left") over a raw date string.

| State | Condition | Classes |
|-------|-----------|---------|
| Expired | date passed | `bg-danger text-white` |
| Expiring soon | 1–7 days | `bg-warning text-dark` |
| Fresh | 8–30 days | `bg-success text-dark` |
| Well stocked | > 30 days | `bg-primary text-white` |

---

## Toolbar / Bulk Actions

- Primary actions (Delete, Archive): always visible
- Secondary filters < 768px: collapse into dropdown (overflow-menu pattern)

---

## Search Bar

- Input must have a visible label (`.visually-hidden` is acceptable)
- Placeholder alone is not sufficient
- Add `role="search"` to the containing element
