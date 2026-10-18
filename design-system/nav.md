# proviant — Navigation Inventory

> Source of truth for the menu structure. Mirrors the shipped app:
> `src/templates/web/partials/nav.tmpl` (sidebar), `src/templates/web/layout/base.tmpl`
> (bottom-nav + mobile more-menu). Labels come from `src/i18n/catalogs/*/nav.json`.
> Stand: 2026-10.

The `design-system/screens/*.html` spec sheets are standalone files (no shared
include), so this document is the single place the menu is defined. When the app
menu changes, change this file, then mirror it in every screen listed below.

## 1. Sidebar — primary items (desktop, ≥ 1024 px)

Rendered in `nav.tmpl` inside `.nav-items`, in this order:

| # | Label (`nav.*` key) | Route | Icon |
|---|---------------------|-------|------|
| 1 | Dashboard (`nav.dashboard`) | `/web` | `bi-speedometer2` |
| 2 | Products (`nav.products`) | `/web/products` | `bi-box-seam` |
| 3 | Shopping List (`nav.shoppingList`) | `/web/shopping-list` | `bi-cart3` |
| 4 | Meal Plan (`nav.mealPlan`) | `/web/mealplan` | `bi-calendar-week` |
| 5 | Recipes (`nav.recipes`) | `/web/recipes` | `bi-journal-richtext` |
| 6 | Waste Analytics (`nav.wasteAnalytics`) | `/web/waste-analytics` | `bi-bar-chart-line` |
| 7 | Settings (`nav.settings`) | `/web/user/settings` | `bi-gear-wide-connected` |

Products carries an optional badge (`.nav-badge`) with the urgent/expiring count.

## 2. Sidebar — footer utilities

| Item | Behaviour | Icon |
|------|-----------|------|
| Theme | Cycles light / dark / system (`localStorage: proviant_theme`) | `bi-circle-half` |
| Notifications | Opens a dropdown, badge with unread count | `bi-bell` |
| Logout | Returns to `/web` | `bi-escape` |
| Version | Links to the matching GitHub release tag | — |
| Source | Links to `github.com/Isotop7/proviant` | `bi-git` |

The theme switcher exists **exactly once** per page: here in the sidebar footer
(desktop) and in the mobile "more" menu. In the spec sheets it sits in the topnav
`.nav-actions` cluster (`.js-theme-toggle`, one delegated listener in the page's
theme script). There is no floating theme button; auth/flow screens (`01`, `09`–`16`)
carry no switcher at all and follow `data-theme` / `prefers-color-scheme`.

## 3. Mobile bottom-nav (< 1024 px)

Rendered in `base.tmpl` as `.bottom-nav`, 5 slots:

| # | Label (`nav.*` key) | Route | Icon |
|---|---------------------|-------|------|
| 1 | Dashboard | `/web` | `bi-speedometer2` |
| 2 | Products | `/web/products` | `bi-box-seam` |
| 3 | Scan (icon-only centre action) | `/web/products/scan` | `bi-upc-scan` |
| 4 | Shopping (`nav.shopping`) | `/web/shopping-list` | `bi-cart3` |
| 5 | Recipes | `/web/recipes` | `bi-book` |

Meal Plan, Waste Analytics and Settings are **not** in the bottom-nav; on mobile
they are reached from the "more" menu (`.mobile-top-more`): Theme, Settings, Logout.

## 4. Mapping into `design-system/screens/`

The spec sheets show the menu as a horizontal **topnav** (`.nav-links`) rather than
the sidebar, so a full 7-column row stays readable at desktop widths. The topnav
carries the same seven items, in the same order, plus the utility cluster
(notifications, theme, logout) in `.nav-actions`. Below 1180 px the link row is
hidden and the bottom tab takes over.

| Screen | Active topnav item | Active bottom tab |
|--------|--------------------|-------------------|
| `02-dashboard.html` | Dashboard | Dashboard |
| `03-products.html` | Products | Products |
| `04-add-product.html` | Products | Products |
| `05-recipes.html` | Recipes | Recipes |
| `06-settings.html` | Settings | — |
| `07-edit-product.html` | Products | Products |
| `08-shopping-list.html` | Shopping List | Shopping |
| `14-user-profile.html` | Settings | — |
| `17-product-detail.html` | Products | Products |
| `18-scan-receipt.html` | Products | Products |
| `19-waste-analytics.html` | Waste Analytics | — |
| `21-mealplan.html` | Meal Plan | — |
| `22-scan-product.html` | — | Scan |

## 5. Token note

The screens currently ship their own inline token block (a green `oklch()` palette),
not the sage palette in `tokens.css` / `DESIGN.md`. Only new screens reuse that
inline block so the whole sheet stays visually consistent. Migrating the screens
onto `tokens.css` is a separate task.
