# Page Override: Home Dashboard (`/web`)

> Overrides apply on top of `MASTER.md`.

## Priority Focus

Glanceable status summary. Users should know their household's food health in <3 seconds.

## Tile Hero Numbers — Status Coloring

Override the generic `text-primary` on all hero numbers. Apply semantic colors:

| Tile | Hero Color |
|------|-----------|
| Total products | `text-primary` (neutral) |
| Expired count | `text-danger` (if > 0), `text-success` (if 0) |
| Expiring soon | `text-warning` (if > 0), `text-success` (if 0) |
| Fresh / OK | `text-success` |

## Empty State (no household)

Current `bi-emoji-dizzy-fill` icon is fine. Ensure:
- Hero icon uses `.hero-icon` class (already defined in SCSS)
- CTA button is `btn-primary` — already correct
- Add `text-muted` to the explanation paragraph
