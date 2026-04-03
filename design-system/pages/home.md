# Page Override: Home Dashboard (`/web`)

> Overrides apply on top of `design-system/MASTER.md`. Only differences from Master are listed.

---

## Priority Focus

Glanceable status summary. Users should know their household's food health in < 3 seconds.

---

## Tile Hero Numbers — Status Coloring

Override the generic `text-primary` on hero numbers. Apply semantic colors:

| Tile | Hero color |
|------|-----------|
| Total products | `text-primary` (neutral) |
| Expired count | `text-danger` (if > 0) / `text-success` (if 0) |
| Expiring soon | `text-warning` (if > 0) / `text-success` (if 0) |
| Fresh / OK | `text-success` |

---

## Empty State (no household yet)

- Hero icon: `.hero-icon` class (defined in SCSS)
- Explanation: `text-muted`
- CTA: `btn-primary`
