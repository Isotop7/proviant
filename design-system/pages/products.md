# Page Override: Products List (`/web/products`)

> Overrides apply on top of `MASTER.md`. Only differences are listed here.

## Priority Focus

This is the **core screen** — users spend most time here. Information density and status readability are the top concerns.

## Layout Override

- Grid: `row-cols-1 row-cols-sm-1 row-cols-md-2 row-cols-lg-3` (MASTER default is lg-3 only — add md-2)
- Card gap: `g-4` (currently `g-5` — reduce slightly for density)

## Status Badge — Expiry Logic

This page's most critical visual element. Expiry state must be **immediately scannable**.

```
date passed          → bg-danger   text-white   "Expired"
1–7 days remaining   → bg-warning  text-dark    "X days left"
8–30 days            → bg-success  text-dark    date string
>30 days             → bg-primary  text-white   date string
```

Consider adding a human-readable countdown for ≤7 days ("3 days left") instead of just a date.

## Toolbar / Bulk Actions

The `productOptionBar` should follow the "overflow-menu" pattern:
- Primary: Delete, Archive buttons (visible)
- If screen <768px: collapse secondary filters into a dropdown

## Search Bar

- Input must have visible label (can be visually hidden with `.visually-hidden`)
- Placeholder alone is not sufficient
- Add `role="search"` to the containing element
