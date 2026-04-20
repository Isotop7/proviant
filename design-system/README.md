# Proviant Design System

## Overview

Proviant is a web application for tracking food products and their expiration dates, helping households reduce food waste. Multiple users share a single household inventory; authentication is required.

**Design system source:** Brief only (no codebase or Figma provided). This system was built from scratch to specification.

---

## Aesthetic Direction

**Tone:** Warm utilitarian. A well-organized kitchen — purposeful, calm, slightly organic. Not a tech startup; a thoughtful household tool. Clean structure with warm undertones.

**One unforgettable thing:** The expiry status system. Five precisely chosen status colors are the visual heartbeat of every screen. Everything else is intentionally restrained to let status pop.

---

## Content Fundamentals

- **Voice:** Direct, calm, practical. Second person ("Your pantry", "You have 3 items expiring"). No exclamation points in alerts. No jargon.
- **Casing:** Sentence case everywhere — labels, buttons, headings. Title case only for the product name "Proviant".
- **Numbers:** Always show counts in context ("3 products expiring" not just "3").
- **Dates:** Relative first ("in 2 days"), absolute on hover/detail ("Apr 22, 2026").
- **Emoji:** Never used.
- **Errors:** Specific and actionable ("Expiry date must be in the future" not "Invalid input").
- **Empty states:** Always explain why and what to do next.

---

## Visual Foundations

### Colors
See `colors_and_type.css` for all CSS custom properties.

- **Background:** Warm parchment white `oklch(0.98 0.012 75)` — avoids harsh pure white
- **Surface:** Slightly cooler card surface `oklch(0.96 0.008 75)`
- **Foreground:** Warm near-black `oklch(0.18 0.022 70)`
- **Secondary text:** Muted warm gray `oklch(0.48 0.018 70)`
- **Accent:** Brand teal-green `oklch(0.50 0.12 162)` — extracted from logo basket outline; used for primary actions
- **Brand slate:** `oklch(0.52 0.08 245)` — extracted from logo wordmark; used as secondary nav color
- **Border:** `oklch(0.88 0.01 75)`

#### Status Colors (core of the product)
| Status | Color | Use |
|--------|-------|-----|
| Expired | Tomato red `oklch(0.55 0.20 25)` | Highest urgency |
| Critical | Burnt orange `oklch(0.64 0.18 45)` | 1–3 days |
| Expiring soon | Amber `oklch(0.72 0.16 80)` | 4–7 days |
| Fresh | Sage green `oklch(0.52 0.10 145)` | >7 days |
| No date | Warm gray `oklch(0.60 0.01 70)` | Unknown |

Status is **always** conveyed with both color AND text — never color alone (WCAG AA requirement).

### Typography
- **Display/UI:** Vend Sans (Google Fonts) — confirmed primary brand font; clean, modern, trustworthy
- **Mono:** DM Mono (Google Fonts) — for dates, barcodes, codes, numeric data
- **Scale:** 12 / 13 / 14 / 16 / 18 / 20 / 24 / 30 / 36px
- **Weights:** 400 (body), 500 (label/emphasis), 600 (heading), 700 (display)

### Spacing
4px base unit. Scale: 4 / 8 / 12 / 16 / 20 / 24 / 32 / 40 / 48 / 64 / 80 / 96px.

### Borders & Radius
- **Inputs, cards:** 8px radius
- **Badges, pills:** 999px (full pill)
- **Buttons:** 8px radius
- **Modals:** 12px radius
- **Border color:** `var(--border)` — 1px solid

### Shadows
- **Card (resting):** `0 1px 3px oklch(0.18 0.02 70 / 0.07), 0 1px 2px oklch(0.18 0.02 70 / 0.04)`
- **Card (hover):** `0 4px 12px oklch(0.18 0.02 70 / 0.10), 0 2px 4px oklch(0.18 0.02 70 / 0.06)`
- **Modal:** `0 20px 60px oklch(0.18 0.02 70 / 0.20)`
- **Dropdown:** `0 8px 24px oklch(0.18 0.02 70 / 0.12)`

### Motion
- **Duration:** 150ms (micro), 200ms (transitions), 300ms (modals)
- **Easing:** `ease-out` for enters, `ease-in` for exits. No bounces.
- **Hover states:** Shadow lift + very subtle scale (1.002) on cards
- **Buttons:** Background color transition 150ms ease-out

### Backgrounds
Solid parchment white. No gradients, no textures, no patterns. Dashboard chart areas use a very subtle `oklch(0.96 0.008 75)` surface.

### Cards
White surface `oklch(1 0 0)`, 8px radius, 1px border `var(--border)`, card-shadow. On hover: shadow lifts. No colored left-border accents.

### Icons
Bootstrap Icons only (CDN: `https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.3/font/bootstrap-icons.css`). Icon size matches text size. Never used as sole conveyors of meaning — always paired with text labels in key UI.

### Imagery
Product images in cards: 64×64px square thumbnails, `object-fit: cover`, 8px radius. Placeholder: striped SVG with "product image" label. No decorative photography.

---

## Iconography

- **System:** Bootstrap Icons exclusively
- **CDN:** `https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.3/font/bootstrap-icons.css`
- **Usage:** `<i class="bi bi-{name}"></i>` inline in HTML
- **Sizes:** Match surrounding text or explicit `font-size`
- **Key icons used:**
  - `bi-box-seam` — products / inventory
  - `bi-speedometer2` — dashboard
  - `bi-archive` — archive
  - `bi-person-circle` — user / settings
  - `bi-upc-scan` — barcode scan
  - `bi-bell` — notifications
  - `bi-trash3` — delete
  - `bi-arrow-counterclockwise` — restore
  - `bi-funnel` — filter
  - `bi-search` — search
  - `bi-plus-lg` — add / create
  - `bi-check2` — confirm / success
  - `bi-exclamation-triangle` — warning
  - `bi-x-circle` — error
  - `bi-info-circle` — info
  - `bi-camera` — barcode scan camera

---

## File Index

```
README.md                    ← This file
SKILL.md                     ← Agent skill descriptor
colors_and_type.css          ← All CSS custom properties (tokens)
assets/
  icon.svg                   ← Approved P lettermark icon (negative space, teal, 96×96)
  header.png                 ← Horizontal lockup: P icon + Vend Sans bold wordmark
preview/
  brand.html                 ← Logo assets + brand colors + Vend Sans specimen
  colors-brand.html          ← Brand color palette
  colors-status.html         ← Status color system
  colors-semantic.html       ← Semantic color tokens
  type-scale.html            ← Typography scale
  type-specimens.html        ← Type specimens (headings, body, mono)
  spacing-tokens.html        ← Spacing & radius tokens
  shadows-elevation.html     ← Shadow / elevation system
  components-buttons.html    ← Button variants & states
  components-badges.html     ← Status badges & chips
  components-inputs.html     ← Form inputs & selects
  components-cards.html      ← Product cards
  components-modals.html     ← Modals (feedback, confirmation)
  components-empty.html      ← Empty states
ui_kits/
  proviant/
    index.html               ← Interactive prototype (React/JSX — design reference only)
    bootstrap-snippets.html  ← Plain HTML + Bootstrap 5 components (copy into Go templates)
    README.md                ← UI kit notes
```
