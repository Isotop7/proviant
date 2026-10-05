# Presets

Every preset is one route of `src/router/router.go:431-474`, logged in as the
seeded demo user unless noted. Output lands in
`.cache/ui-captures/<preset>-<timestamp>/`.

| Preset | Route | Login | Ready selector (waits for) |
|---|---|---|---|
| `login` | `/web/auth` | no | `#authForm` |
| `dashboard` | `/web` | yes | `#dashboard-status` reading `Dashboard loaded` |
| `products` | `/web/products` | yes | `#productRows .list-row` |
| `products-expired` | `/web/products?status=expired` | yes | `#productRows .list-row` |
| `products-consumed` | `/web/products?status=archived` | yes | `#productRows .list-row` |
| `products-grid` | `/web/products?view=grid` | yes | `.product-card` |
| `product-detail` | `/web/products/1/view` | yes | `.text-bg-tertiary h2` |
| `product-edit` | `/web/products/1/edit` | yes | `#editProductForm` |
| `add-product` | `/web/products/scan` | yes | `#viewfinder` |
| `recipes` | `/web/recipes` | yes | `.recipe-card`, or the error container becoming visible |
| `waste` | `/web/waste-analytics` | yes | `#waste-status` reading `Waste analytics loaded` |
| `shopping-list` | `/web/shopping-list` | yes | `#shoppingListContainer .sl-list-row` |
| `settings` | `/web/user/settings` | yes | `h1` reading `User Settings` |
| `user` | `/web/user` | yes | `h3` reading `User` |
| `onboarding` | `/web/onboarding` | yes | `#step1` |
| `forgot-password` | `/web/forgot-password` | no | `#forgot-password-form` |
| `reset-password` | `/web/reset-password?token=demo-verify-token` | no | `#reset-password-form` |
| `invite-accept` | `/web/invite/accept?token=demo-invite-token` | no | `.alert-info` |
| `verify-email` | `/web/verify-email?token=demo-verify-token` | no | spinner, then success or failure alert |
| `unsubscribe` | `/web/unsubscribe?token=demo-unsubscribe-token` | no | `h1` reading `Unsubscribed` |
| `not-found` | `/web/this-page-does-not-exist` | yes | `.display-5` reading `Something went wrong` |
| `all` | walk of the twelve above | yes | — |

`login`, `forgot-password`, `reset-password`, `invite-accept`, `verify-email` and
`unsubscribe` are the presets reachable without a session; every other preset logs
in first.

## Token-gated public pages

`invite-accept`, `verify-email` and `unsubscribe` render an error card when the
`token` query parameter is missing or invalid, so a token-free preset would only
ever document the failure branch. `seed.go` writes the three tokens those presets
use:

| Constant | Token | Branch it unlocks |
|---|---|---|
| `DemoInviteToken` | `demo-invite-token` | Pending household invitation → the "You've been invited" card |
| `DemoVerifyToken` | `demo-verify-token` | Pending email verification → the client-side success card |
| `DemoUnsubscribeToken` | `demo-unsubscribe-token` | Mail-digest unsubscribe → the success card |

Both public branches are destructive in the live app: accepting an invitation
moves the user into another household, verifying flips the account to verified,
unsubscribing sets `MailDigestFrequency` to disabled and deletes the token. None
of that matters here because each run reseeds, but the same tokens must not be
pointed at a real deployment.

The demo user's own `EmailVerification` row is not used for the `verify-email`
preset — the demo account is already verified, so the endpoint answers "email
address already verified". The seed creates a fourth user, `dana`, holding the
pending verification instead.

## Why the ready selectors are not `<h1>`

Most pages fetch their content after `DOMContentLoaded`, so "the page loaded"
says nothing about whether it is worth photographing:

- `/web` — `homeStats.js:97` fetches stats, streak, savings, household settings
  and the activity feed in sequence, then constructs three Chart.js instances.
  It sets `#dashboard-status` to `Dashboard loaded` at the end (`homeStats.js:325`).
- `/web/waste-analytics` — same pattern, `#waste-status` at `wasteAnalytics.js:293`.
- `/web/recipes` — the container is server-rendered empty and filled by a fetch,
  so the selector accepts either the cards or the error container.
- `/web/shopping-list` — the rows are client-rendered from
  `shopping_list_items`, so the selector waits for the first one rather than the
  add-item form, which renders in every state including the empty one.

## `--url` has no readiness contract

A preset's ready selector describes *its* page, so applying it to an arbitrary
`--url` target just times out on a missing element. `--url` therefore skips the
selector entirely and relies on `shot()`'s network-idle wait, fonts wait and
settle. If the target fetches its content late enough to matter, use a custom
flow with an explicit `settle()` instead.

The still is named after the route actually visited, not the preset:
`--url /web/products/7/view` → `01-web-products-7-view.png`.

## Things a preset cannot show you

- **Overlays.** Modals, offcanvas panels and dropdowns need a click. Use a custom
  flow (`--script`) — see `scripts/flows/` for `overlays.mjs`,
  `mobile-filters.mjs` and `onboarding-steps.mjs`.
- **Onboarding steps 2 and 3.** `/web/onboarding` restores from the user's
  `OnboardingState`, and the demo state is empty, so the route always lands on
  step 1.
- **Products other than id 1.** The demo seed creates 30 active products with
  predictable ids plus 22 archived ones from id 31, so `/web/products/<n>/view` is
  reachable with `--url`.

## Camera-dependent presets

`add-product` (`/web/products/scan`) is **degraded in headless Chromium**:
`html5-qrcode` finds no camera, so the viewfinder never produces a stream. What
you get is the page chrome plus the manual-entry affordances. That is the honest
headless capture, not a bug in the tool.

The camera *tab* inside the add-product modal behaves the same way, and reports
the failure through the shared `#proviantFeedbackModal` — `flows/overlays.mjs`
captures that stack rather than pretending it works.

## The seed is a capture dependency

These pages are only worth photographing because `seed.go` populates what they
read:

| Page | Seed rows it needs |
|---|---|
| `waste` | `savings_records` over a ~6 month window, `products` with `categories` |
| `shopping-list` | `shopping_list_items`, plus `min_stock_amount` for the import banner |
| `dashboard` | `activity_logs`, `waste_streaks`, `savings_records` |
| `products-consumed` | soft-deleted `products` with `removal_reason` |
| `products-expired` | a product past its `expire_at` (seed id 7) |
| `product-detail` restock modal | ≥2 consumed samples in the 90-day rate window |

Adding a page to `PUBLISHED_SCREENS` in `scripts/uicapture.mjs` without adding its
rows to the seed produces a screenshot of an empty feature.

## Stills are viewport-sized by default

A still is what a user sees in a window, so it matches the viewport exactly:
1440×900 at `desktop`, 390×844 at `mobile`. This is not just a preference — the
committed `screenshots/*.png` were already viewport-sized (1658×1032 and
398×870), and `position: fixed` chrome (the sidebar, the mobile tab bar, modal
backdrops) can only render correctly when the capture is one viewport tall.

`--full-page` opts into the whole document. Two things have to be neutralised
first, or the result is worse than useless:

1. `main.scss:285-291` pins `html, body { height: 100%; min-height: 100vh }` on
   mobile, which caps `documentElement.scrollHeight` at one viewport while
   `body.scrollHeight` keeps growing. Playwright sizes the capture from the
   overflowing body and paints well over a thousand pixels of nothing.
2. `position: fixed` chrome renders once at its viewport offset, so it ends up
   floating in the middle of an otherwise scrolled page.

`shot()` therefore injects `html, body { height: auto; overflow: visible }` and
re-anchors every fixed element measured **before** that injection — measuring
afterwards reports the post-override geometry. Placement follows three rules: a
rail or backdrop that filled the viewport is stretched to span the document, a bar
flush with the viewport bottom is pinned to the document bottom, and anything
else keeps the offset the user actually saw.

## Known layout defect

`/web/products/1/edit` overflows its viewport by 10 px at `mobile`: the sticky
action footer (`productsEdit.tmpl:308-327`) holds four children in a
non-wrapping `d-flex gap-2` row whose intrinsic width exceeds 390 − 28 px of
padding. The trash button hangs past the right edge. It is invisible in a
viewport-sized still — the footer sits behind the tab bar — and only shows up in
`--full-page`, where it widens the PNG to 400 px. Pre-existing, unrelated to the
capture tool, and left alone here.

## Full-page captures and lazy images

`loading="lazy"` images below the fold never load unless something scrolls the
page. `shot()` therefore walks the document top-to-bottom and back before
capturing (`primeLazyContent` in `scripts/browser-capture.mjs`) — on every shot,
not just full-page ones, because the scroll also settles sticky headers.

Overlays are the other full-page hazard: a `position: fixed` modal keeps its own
height while the document grows beneath it. The default viewport-sized still
avoids the problem entirely, which is the other reason it is the default.