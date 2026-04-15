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

For ≤ 7 days remaining, show both a colored badge and an urgency text line on the card.

| State | Condition | Badge classes | Urgency text |
|-------|-----------|---------------|--------------|
| Expired | date passed | `bg-danger text-white` | "Expired today" / "Expired N day(s) ago" |
| Critical | ≤ 3 days | `bg-danger text-white` | "Expires tomorrow" / "Expires in N days" |
| Expiring soon | 4–7 days | `bg-warning text-dark` | "Expires in N days" |
| Fresh | > 7 days | `bg-success text-dark` | — |
| No date | — | `bg-secondary` | — |

Urgency text uses `text-danger` for expired/critical, `text-warning` for expiring soon. Rendered below the expiry badge row on the product card.

## Default Sort Order

Products sorted by `expire_at ASC` by default — expired and critical products appear at the top. Products with no expiry date set appear at the end.

---

## Toolbar / Bulk Actions

- Primary actions (Delete, Archive): always visible
- Secondary filters < 768px: collapse into dropdown (overflow-menu pattern)

---

## Search Bar

- Input must have a visible label (`.visually-hidden` is acceptable)
- Placeholder alone is not sufficient
- Add `role="search"` to the containing element
