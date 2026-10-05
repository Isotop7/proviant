---
name: ui-capture
description: >-
  Capture PNG screenshots and a webm recording of the live Proviant UI from the
  working tree. Use when asked for a "screenshot", "screen capture", "record the
  UI", "demo video", "what does the UI look like", "visual check", or a GIF/MP4 of a
  page. Boots its own hermetic server with demo data, so it never touches
  src/data/, src/config.yaml, or an already-running dev server.
---

# ui-capture

Boots a throwaway Proviant instance from this working tree, drives it with
headless Chromium in the Playwright container, and writes PNG stills plus a webm
recording to a gitignored directory.

Nothing outside `.cache/ui-captures/` is written: the server runs from a temp
directory against a temp SQLite database and a temp copy of the config.

## When to use

| You want | Use |
|---|---|
| One page as a PNG | `--preset <name>` |
| A look at the whole UI | `--preset all` |
| A still of one specific route | `--preset dashboard --url /web/products` |
| Modals, dropdowns, a click-through | `--script <flow.mjs>` |
| A clip someone can paste into a PR or issue | `--preset X --gif` |
| An mp4 for a demo page | `--preset X --mp4` |
| Stills only, fastest | `--preset X --no-video` |
| Refresh the committed screenshots and the README gallery | `--publish` |

Presets are per-route and cannot reach overlay states. Anything needing a click
needs a custom flow.

## One-time setup

```bash
podman pull mcr.microsoft.com/playwright:v1.63.0-noble
```

`playwright` is a skill-local dependency, not a repo one: the runner installs
`playwright@1.63.0` into `.kilo/skills/ui-capture/node_modules` on first use
(`--ignore-scripts`, so no browser download — the image has the browsers). As a
repo devDependency it was installed by every Docker build and every CI job for a
tool no build step invokes. `.kilo/.gitignore` keeps that directory out of git.

The tag must equal the installed `playwright` version — `playwright-core` refuses
a mismatched browser revision. The pin lives in `PLAYWRIGHT_VERSION` in
`scripts/uicapture.mjs` and the runner derives the tag from the package it
installed. If you bump the pin, pull the new tag.

The runner never pulls mid-run. A missing image exits 9 and prints the command.

## Commands

```bash
# Desktop still + webm of the dashboard
node .kilo/skills/ui-capture/scripts/uicapture.mjs --preset dashboard

# Every page, phone width, plus a shareable GIF
node .kilo/skills/ui-capture/scripts/uicapture.mjs --preset all --viewport mobile --gif

# Stills only (stills are viewport-sized by default)
node .kilo/skills/ui-capture/scripts/uicapture.mjs --preset products --no-video

# Stills that span the whole document instead
node .kilo/skills/ui-capture/scripts/uicapture.mjs --preset products --full-page

# Refresh screenshots/ and the README gallery
node .kilo/skills/ui-capture/scripts/uicapture.mjs --publish

# One route, exactly
node .kilo/skills/ui-capture/scripts/uicapture.mjs --preset dashboard --url /web/products/7/view

# A click-through
node .kilo/skills/ui-capture/scripts/uicapture.mjs \
  --script .kilo/skills/ui-capture/scripts/flows/example.mjs

# Alongside a running `task run`
node .kilo/skills/ui-capture/scripts/uicapture.mjs --preset all --port 0
```

Every produced file is printed as an absolute path, one per line, followed by a
count. Default output: `.cache/ui-captures/<preset>-<YYYYMMDD-HHMMSS>/`.

## Refreshing the README gallery

```bash
node .kilo/skills/ui-capture/scripts/uicapture.mjs --publish
```

One command, no arguments. It shoots seven screens at both viewports, copies
them over `screenshots/`, and rewrites the generated gallery in `README.md`.

**Stills, not video.** GitHub strips a `<video>` tag with a repo-relative
`src`, so an mp4 in the repo only renders from an absolute URL; a GIF of the
full walk would add megabytes to git for something a reader cannot pause. The
README already documents itself with viewport-sized desktop/mobile PNG pairs, so
that is what it keeps producing.

**Where it writes.** `screenshots/portal.png`, `search.png`, `create.png`,
`recipe.png`, `waste.png`, `shopping.png` and `login.png`, plus `_mobile`
variants for the first six. The four original filenames are reused from the
versions already committed, so existing links keep resolving.

**Where in the README.** One delimited region inside `## Screenshots`:

```markdown
<!-- ui-capture:begin — regenerate with `node .kilo/skills/ui-capture/scripts/uicapture.mjs --publish` -->
…desktop/mobile tables…
<!-- ui-capture:end -->
```

Only what sits between the markers is rewritten; everything else in the file is
left alone. On a first run the block is inserted directly under the
`## Screenshots` heading. Republishing is idempotent — the second run produces a
byte-identical README.

`## Preview` near the top of the README is **not** generated. Those two embeds
are the above-the-fold pitch and stay hand-curated; they point at the same
files, so a publish keeps them current without touching that section.

Review the diff. A publish legitimately rewrites tracked binaries, so it belongs
in its own commit rather than being mixed into a feature change.

### Choosing screens

Only pages the demo seed actually populates belong in the gallery. The trap is
that a page can be fully implemented and still render empty: `/web/waste-analytics`
read `savings_records`, which `seed.go` did not create, so it showed
`€0.00 wasted` across every tile and an empty category list — a gallery entry
advertising the feature as broken. `seed.go` now writes ~6 months of
consume/waste events, shopping-list items, an activity feed and a waste streak,
so `waste` and `shopping-list` earn their place.

Before adding a page to `PUBLISHED_SCREENS` in `scripts/uicapture.mjs`, check it
reads rows the seed writes. `references/presets.md` has the page-to-table table.
That array is the only place a gallery screen is declared: the runner passes it
to the browser stage, which shoots exactly it, and `publishScreens` copies
exactly it. A still that no entry claims is an error, not a silent no-op.

## Presets

`scripts/presets.mjs` holds the table. `uicapture.mjs --help` and
`references/presets.md` both describe it; the route table, ready selectors, and
their known limitations — including the degraded camera state of `add-product`
and a 10 px horizontal overflow on `/web/products/1/edit` — are in
`references/presets.md`.

## Custom flows

A flow is one file with a default export. It gets helpers and imports nothing, so
the same file runs under container Node and host Node. The runner has already
logged in as `demo` before the flow starts.

```js
// .kilo/skills/ui-capture/scripts/flows/x.mjs
export default async function flow({ page, shot, goto, settle }) {
  await goto('/web/products');
  await settle('#productRows .list-row');
  await shot('products');

  await page.click('#btnOpenAddProductModal');
  await settle('#addProductModal.show', 'visible');
  await shot('add-product-modal');
}
```

| Helper | What it does |
|---|---|
| `shot(name)` | Kill animations, wait for fonts and network, scroll to prime lazy images, write `<NN>-<name>.png` |
| `goto(path)` | Navigate; throws with a login diagnosis if the router redirects to `/web/auth` |
| `settle(selector, state?)` | Wait for proof the page finished rendering — not merely that it loaded |
| `page` | The raw Playwright page |
| `baseURL`, `viewport`, `clip`, `login()` | Context |

`shot()` returns the file path and takes an optional second argument, `raw`,
which drops the numeric prefix for stable filenames — that is how `--publish`
gets `portal.png` instead of `01-portal.png`.

A flow must live inside the repo, because container mode mounts only the repo.

Three flows ship with the skill:

| Flow | Covers |
|---|---|
| `flows/overlays.mjs` | Add-product modal (manual + camera-unavailable), export dropdown, bulk selection, cook modal, destructive confirm, notifications dropdown, restock suggestion, create-token modal |
| `flows/mobile-filters.mjs` | The mobile filter sheet; needs `--viewport mobile` or it throws |
| `flows/onboarding-steps.mjs` | Onboarding steps 1–3 and all three household sub-modes |

## Determinism

Two runs of the same preset produce byte-identical stills. That is the regression
test for visual changes — if a diff appears when nothing changed, something
non-deterministic leaked into the render.

It holds because each run gets a fresh browser profile, the viewport is
viewport-sized rather than full-page (a stable, fixed raster), `document.fonts.ready`
is awaited, `Chart.defaults.animation` is forced off before
`chart.umd.min.js` executes, all CSS animations and transitions are disabled
immediately before each capture, and locale and timezone are pinned to
`en-US`/`UTC`.

Date-derived labels are relative to the seed, so they are stable within a day.
A re-run on a later date legitimately differs.

## Failure modes

| Code | Condition | Fix |
|---|---|---|
| 2 | Port already in use | Stop the holder, pass `--port <n>` or `--port 0` |
| 3 | No compiled CSS, no config source, or playwright could not be installed | `task css`; `cp src/config.yaml.sqlite.tmpl src/config.yaml`; or run the printed `npm install --prefix` |
| 4 | `--browser local` with no Chromium | See `references/fedora-setup.md` |
| 5 | Server never became ready, or a publish still was not claimed | Read the temp server log from the printed path; check `PUBLISHED_SCREENS` |
| 6 | Login rejected | The seed did not run, or the demo password changed |
| 7 | Selector or URL timeout | The named step never appeared; earlier stills are kept |
| 8 | No podman and no docker | Install podman, or use `--browser local` |
| 9 | Image not present locally | Run the printed `podman pull` |
| 10 | `chromium.launch()` failed | Re-run; the runner reinstalls playwright, then re-pull the derived tag |

Exit 0 with `--mp4`/`--gif` means ffmpeg was missing or lacked an H.264
encoder; the webm and all stills are intact. The runner probes
`libx264 → libopenh264 → h264_vaapi` because Fedora's `ffmpeg-free` ships
without libx264 — `references/fedora-setup.md` has the dnf escape hatch.

The server is torn down in a `finally` on every exit path, so a leaked
`proviant` holding the port never happens.

## Do not

- **Do not hand-edit `screenshots/` or the generated README block.** `--publish`
  owns both. Editing them by hand means the next publish silently reverts your
  change, or worse, keeps it and drifts.
- **Do not add a `Taskfile.yml` task for this.** The skill lives under `.kilo/`,
  so a task would be broken for anyone without it.
- **Do not point `--out` at `screenshots/`.** A normal run writes numbered stills
  and a video there; that directory is owned by `--publish`, which writes stable
  filenames.

## Reference

- `references/presets.md` — route table, ready selectors, per-preset caveats
- `references/fedora-setup.md` — image pull, tag derivation, SELinux, ffmpeg
  codec gaps, `--browser local`
- `scripts/uicapture.mjs` — host orchestrator: preflight, build, seed, config,
  server lifecycle, container launch, transcodes, teardown
- `scripts/browser-capture.mjs` — browser only: launch, login, flow, stills, video