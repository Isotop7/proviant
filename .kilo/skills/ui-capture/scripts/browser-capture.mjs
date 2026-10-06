// Browser stage of the ui-capture skill.
//
// Runs unchanged under the Playwright container's Node or under host Node
// (`--browser local`). It knows nothing about Go, containers, or the temp
// server: the orchestrator (uicapture.mjs) hands it a running BASE_URL and an
// output directory, and this file produces PNG stills plus a webm recording.
//
// Driven entirely by environment variables:
//   BASE_URL  required  http://127.0.0.1:5114
//   OUT_DIR   required  absolute directory to write artifacts into
//   VIEWPORT  required  WxH
//   CLIP      optional  1 = viewport-sized stills (default), 0 = full page
//   VIDEO     optional  1 = record webm
//   PRESET    required  preset name, see PRESETS below (unused in publish mode)
//   ARTIFACT_NAME  optional  base name for the webm (defaults to PRESET)
//   PUBLISH   optional  1 = shoot the README screen set at both viewports
//   PUBLISH_SCREENS  required when PUBLISH=1 — JSON screen list from uicapture.mjs
//   TARGET    optional  path overriding the preset's path (from --url)
//   FLOW      optional  absolute path to a custom flow module (from --script)
//   TIMEOUT   optional  per-step timeout in ms (default 45000)
//
// Exit codes match uicapture.mjs so failures stay attributable:
//   4  Chromium not installed (local mode only)
//   6  login rejected / session lost
//   7  selector or URL timeout inside a flow (stills so far are kept)
//  10  chromium.launch() failed (playwright / image version mismatch)

import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const EXIT_LOGIN = 6;
const EXIT_FLOW = 7;
const EXIT_MISSING_BROWSER = 4;
const EXIT_BROWSER = 10;

const BASE_URL = requireEnv('BASE_URL').replace(/\/+$/, '');
const OUT_DIR = requireEnv('OUT_DIR');
const PRESET = requireEnv('PRESET');
const ARTIFACT_NAME = process.env.ARTIFACT_NAME || PRESET;
const PUBLISH = process.env.PUBLISH === '1';
const VIEWPORT = parseViewport(requireEnv('VIEWPORT'));
const TARGET = process.env.TARGET || '';
const FLOW = process.env.FLOW || '';
const CLIP = process.env.CLIP !== '0';
const VIDEO = process.env.VIDEO !== '0';
const TIMEOUT = Number.parseInt(process.env.TIMEOUT || '45000', 10);

// Publish mode always shoots a fixed desktop/mobile pair, independent of
// --viewport, because the two sizes are a property of the README layout rather
// than of the run.
const MOBILE_VIEWPORT = { width: 390, height: 844 };

// The preset table lives in presets.mjs so uicapture.mjs can build its --help
// list from the same data this file shoots.
const { PRESETS, ALL_WALK } = await import('./presets.mjs');

// The README screen set arrives from the orchestrator (PUBLISH_SCREENS in
// uicapture.mjs PUBLISHED_SCREENS) so the shoot list and the publish list cannot
// drift apart. `anonymous` screens are shot before anything authenticates:
// /web/auth redirects a logged-in user to /web, so login needs its own session.
const PUBLISH_SCREENS = PUBLISH ? parsePublishScreens() : [];

// Determinism is a hard requirement: Chart.js on /web and /web/waste-analytics
// animates on every load, so a screenshot taken mid-animation is garbage. The
// init script has to run *before* chart.umd.min.js executes, and `window.Chart`
// does not exist yet at that point — hence the property setter, which patches
// Chart.defaults the moment the UMD bundle assigns its global.
const CHART_QUIET = `(() => {
  const quiet = (C) => {
    if (!C || !C.defaults) return;
    C.defaults.animation = false;
    if (C.defaults.transitions && C.defaults.transitions.active) {
      C.defaults.transitions.active.animation.duration = 0;
    }
  };
  let chart;
  Object.defineProperty(window, 'Chart', {
    configurable: true,
    get() { return chart; },
    set(value) { chart = value; quiet(value); },
  });
})();`;

// Recipe cards use loading="lazy", so a screenshot taken without ever scrolling
// leaves everything below the fold blank. Walk the page once, then return to the
// top before the capture.
//
// These page-side helpers are real functions, not strings passed to
// page.evaluate: a string expression evaluates to a function object whose
// return value and promise Playwright does not await, which silently breaks
// both the lazy-load wait and the re-anchor bookkeeping.
async function primeLazyContent() {
  const step = Math.max(window.innerHeight, 400);
  for (let y = 0; y < document.body.scrollHeight; y += step) {
    window.scrollTo(0, y);
    await new Promise((resolve) => requestAnimationFrame(() => setTimeout(resolve, 60)));
  }
  window.scrollTo(0, 0);
}

const QUIET_CSS = '*,*::before,*::after{animation:none!important;transition:none!important}';

// Full-page captures are structurally hostile to this app's shell, for two
// independent reasons. Both are neutralised here, and only for full-page shots.
//
// 1. main.scss:285-291 pins `html, body { height: 100%; min-height: 100vh }` on
//    mobile. That caps documentElement.scrollHeight at one viewport while
//    body.scrollHeight keeps growing, so Playwright sizes the capture from the
//    overflowing body and paints ~1300px of nothing below the fold.
// 2. The sidebar, the mobile tab bar and every modal backdrop are
//    `position: fixed`, i.e. positioned against the viewport. In a capture that
//    is much taller than the viewport they render once at their viewport offset,
//    so the chrome floats in the middle of an otherwise scrolled page.
const FULLPAGE_CSS = `
html, body {
  height: auto !important;
  min-height: 0 !important;
  overflow: visible !important;
}
`;

// Re-anchor fixed chrome to its document offset.
//
// Measurement MUST happen before FULLPAGE_CSS is injected: `html, body {height:
// auto}` changes how a `height: 100%` sidebar resolves, so measuring afterwards
// reports the post-override geometry and the viewport-filling test below starts
// guessing. So: measure and stash, inject, then apply.
function measureFixed() {
  window.__uicFixed = [...document.querySelectorAll('body *')]
    .filter((el) => getComputedStyle(el).position === 'fixed')
    .map((el) => {
      const r = el.getBoundingClientRect();
      return { el, top: r.top + window.scrollY, left: r.left, width: r.width, height: r.height };
    })
    .filter((m) => m.width > 0 || m.height > 0);
  return window.__uicFixed.length;
}

// Placement rules, applied to the geometry measured before FULLPAGE_CSS:
//   viewport-filling  a rail or backdrop that filled the viewport → stretch to
//                     the document, so it reads as continuous chrome
//   bottom-anchored   a bar flush with the viewport bottom (mobile tab bar,
//                     sticky action footer) → pin to the document bottom,
//                     which is what a designer would export
//   otherwise         keep the document offset the user actually saw
function applyFixed() {
  const viewportHeight = window.innerHeight;
  const docHeight = document.documentElement.scrollHeight;
  for (const m of window.__uicFixed) {
    m.el.dataset.uicFixed = JSON.stringify({
      position: m.el.style.position, top: m.el.style.top,
      left: m.el.style.left, width: m.el.style.width, height: m.el.style.height,
    });
    m.el.style.position = 'absolute';
    m.el.style.left = `${m.left}px`;

    if (m.height >= viewportHeight - 2) {
      m.el.style.top = `${m.top}px`;
      m.el.style.height = `${Math.max(m.height, docHeight - m.top)}px`;
      m.el.style.width = `${m.width}px`;
    } else if (m.top + m.height >= viewportHeight - 2) {
      m.el.style.top = `${docHeight - m.height}px`;
    } else {
      m.el.style.top = `${m.top}px`;
    }
  }
  return window.__uicFixed.length;
}

function restoreFixed() {
  for (const m of window.__uicFixed || []) {
    const saved = JSON.parse(m.el.dataset.uicFixed);
    m.el.style.position = saved.position;
    m.el.style.top = saved.top;
    m.el.style.left = saved.left;
    m.el.style.width = saved.width;
    m.el.style.height = saved.height;
    delete m.el.dataset.uicFixed;
  }
  window.__uicFixed = [];
}

function requireEnv(name) {
  const value = process.env[name];
  if (!value) {
    process.stderr.write(`uicapture: missing required env ${name}\n`);
    process.exit(EXIT_BROWSER);
  }
  return value;
}

function parseViewport(raw) {
  const match = /^(\d+)x(\d+)$/.exec(raw.trim());
  if (!match) {
    process.stderr.write(`uicapture: VIEWPORT must look like 1440x900, got ${raw}\n`);
    process.exit(EXIT_BROWSER);
  }
  return { width: Number(match[1]), height: Number(match[2]) };
}

function parsePublishScreens() {
  let screens;
  try {
    screens = JSON.parse(requireEnv('PUBLISH_SCREENS'));
  } catch (error) {
    process.stderr.write(`uicapture: PUBLISH_SCREENS is not valid JSON: ${error.message}\n`);
    process.exit(EXIT_BROWSER);
  }
  const usable = screens.filter((entry) => PRESETS[entry.preset]);
  const unknown = screens.filter((entry) => !PRESETS[entry.preset]).map((entry) => entry.preset);
  if (unknown.length > 0) {
    // The orchestrator's list names a preset this file does not define. Fail
    // rather than skip: a silently dropped screen leaves a stale screenshot in
    // screenshots/ with no signal that the run missed it.
    process.stderr.write(`uicapture: unknown publish preset(s): ${unknown.join(', ')}\n`);
    process.exit(EXIT_FLOW);
  }
  return usable;
}

function log(message) {
  process.stderr.write(`[capture] ${message}\n`);
}

// /web/products/7/view -> web-products-7-view
function routeSlug(route) {
  return route.replace(/^\/+|\/+$/g, '').replace(/[^a-z0-9]+/gi, '-').replace(/^-|-$/g, '').toLowerCase() || 'page';
}

// UiCaptureError carries the exit code the orchestrator should surface.
class UiCaptureError extends Error {
  constructor(code, message) {
    super(message);
    this.code = code;
  }
}

async function main() {
  const preset = PRESET === 'all' ? null : PRESETS[PRESET];
  if (PRESET !== 'all' && !preset && !PUBLISH) {
    throw new UiCaptureError(EXIT_FLOW, `unknown preset "${PRESET}" (known: ${Object.keys(PRESETS).join(', ')}, all)`);
  }
  if (PRESET === 'all' && TARGET) {
    throw new UiCaptureError(EXIT_FLOW, '--url cannot be combined with --preset all; use a single preset');
  }
  if (PUBLISH && (FLOW || TARGET)) {
    throw new UiCaptureError(EXIT_FLOW, 'publish mode runs a fixed screen set; drop --script/--url');
  }

  await fs.mkdir(OUT_DIR, { recursive: true });

  let chromium;
  try {
    ({ chromium } = await import('playwright'));
  } catch (error) {
    throw new UiCaptureError(
      EXIT_BROWSER,
      `cannot import the playwright npm package: ${error.message}\n` +
        '  Run `npm install` in the repo root.',
    );
  }

  let browser;
  try {
    browser = await chromium.launch({
      // Playwright defaults this to false, which passes --no-sandbox and leaves
      // Chromium as an unsandboxed root process. The sandbox works in the
      // Playwright image (uicapture.mjs drops all capabilities, so the seccomp
      // sandbox is what keeps a renderer compromise contained).
      chromiumSandbox: true,
      args: [
        // Deterministic raster: no GPU-dependent AA differences between runs.
        '--disable-lcd-text',
        '--force-color-profile=srgb',
        '--font-render-hinting=none',
        '--disable-partial-raster',
        '--disable-skia-runtime-opts',
        '--hide-scrollbars',
      ],
    });
  } catch (error) {
    // "Executable doesn't exist" means the npm package is installed but no
    // browser was downloaded — the local-mode failure (exit 4). Anything else
    // in container mode is a version mismatch.
    const missingBrowser = /Executable doesn't exist|playwright install/i.test(error.message);
    throw new UiCaptureError(
      missingBrowser ? EXIT_MISSING_BROWSER : EXIT_BROWSER,
      `chromium.launch() failed: ${error.message}\n` +
        (missingBrowser
          ? '  Run `npx playwright install chromium` and install the Fedora browser\n' +
            '  libraries — see references/fedora-setup.md.\n'
          : '  In container mode this is almost always a playwright / image version mismatch:\n' +
            '  re-run `npm install`, then re-pull the tag uicapture prints for that version.\n'),
    );
  }

  const videoDir = VIDEO && !PUBLISH ? await fs.mkdtemp(path.join(os.tmpdir(), 'uicapture-video-')) : undefined;
  const sessions = [];
  let counter = 0;

  // One context per viewport. Playwright fixes the viewport per context, so a
  // desktop/mobile sweep needs two sessions rather than a mid-run resize — and
  // the helpers below close over their own page, which keeps that clean.
  const openSession = async (size) => {
    const context = await browser.newContext({
      viewport: size,
      // Full-page stills of a 1440px-wide viewport at DPR 2 would be 2x the
      // pixels for no benefit; pin it so a WxH viewport really is WxH.
      deviceScaleFactor: 1,
      colorScheme: 'light',
      reducedMotion: 'reduce',
      // Date formatting is locale- and zone-dependent; pinning both keeps
      // "re-run and diff the PNGs" a meaningful regression check.
      locale: 'en-US',
      timezoneId: 'UTC',
      recordVideo: videoDir ? { dir: videoDir, size } : undefined,
      serviceWorkers: 'block',
    });
    context.setDefaultTimeout(TIMEOUT);
    context.setDefaultNavigationTimeout(TIMEOUT);
    await context.addInitScript(CHART_QUIET);

    const page = await context.newPage();
    const session = { context, page, size };

    session.goto = async (route) => {
      log(`goto ${route}`);
      await page.goto(`${BASE_URL}${route}`, { waitUntil: 'domcontentloaded' });
      // An expired/absent JWT makes the router 307 to /web/auth. Catching it
      // here gives a login diagnosis instead of a mystery screenshot.
      if (route !== '/web/auth' && new URL(page.url()).pathname === '/web/auth') {
        throw new UiCaptureError(EXIT_LOGIN, `redirected to /web/auth while opening ${route} — session is not authenticated`);
      }
    };

    session.settle = async (readySelector, state = 'attached') => {
      if (!readySelector) return;
      await page.waitForSelector(readySelector, { state });
    };

    // `raw` skips the numeric prefix, so publish mode gets stable filenames
    // instead of 01-portal.png.
    session.shot = async (name, raw = false) => {
      // Re-inject per shot: a navigation discards the previously added <style>.
      await page.addStyleTag({ content: QUIET_CSS });
      if (!CLIP) {
        await page.evaluate(measureFixed);
        await page.addStyleTag({ content: FULLPAGE_CSS });
        log(`full-page: re-anchored ${await page.evaluate(applyFixed)} fixed element(s)`);
      }
      await page.evaluate(primeLazyContent);
      await page.evaluate(async () => {
        if (document.fonts) await document.fonts.ready;
      });
      await page.waitForLoadState('networkidle', { timeout: 8000 }).catch(() => {});
      await page.waitForTimeout(400);
      const prefix = raw ? '' : `${String(counter += 1).padStart(2, '0')}-`;
      const file = path.join(OUT_DIR, `${prefix}${name}.png`);
      await page.screenshot({ path: file, fullPage: !CLIP });
      if (!CLIP) await page.evaluate(restoreFixed);
      log(`shot ${path.basename(file)}`);
      return file;
    };

    session.login = async () => {
      log('login as demo/demo');
      await page.goto(`${BASE_URL}/web/auth`, { waitUntil: 'domcontentloaded' });
      await page.waitForSelector('#authForm');
      await page.fill('#auth-username', 'demo');
      await page.fill('#auth-password', 'demo');
      await page.click('#btn-auth-submit');
      try {
        await page.waitForURL(/\/web(\/onboarding)?$/, { timeout: TIMEOUT });
      } catch {
        // auth.js:203 shows a Bootstrap modal on 401 and never navigates, so
        // the URL we are still sitting on distinguishes the two failures.
        if (new URL(page.url()).pathname !== '/web/auth') {
          throw new UiCaptureError(
            EXIT_FLOW,
            `login landed on ${page.url()} instead of /web or /web/onboarding`,
          );
        }
        // auth.js:189 also stalls here when getOnboardingState rejects.
        const title = await page.textContent('#proviantFeedbackTitle', { timeout: 1500 }).catch(() => '');
        const detail = await page.textContent('#proviantFeedbackMessage', { timeout: 1500 }).catch(() => '');
        throw new UiCaptureError(
          EXIT_LOGIN,
          `login rejected (feedback: ${(title || '').trim() || 'none'} / ${(detail || '').trim() || 'none'}) — ` +
            'check that seed.go ran and that demo/demo still exists',
        );
      }
      log(`logged in, landed on ${new URL(page.url()).pathname}`);
    };

    session.helpers = {
      page, goto: session.goto, settle: session.settle, shot: session.shot,
      login: session.login, baseURL: BASE_URL, viewport: size, clip: CLIP,
    };

    sessions.push(session);
    return session;
  };

  try {
    if (PUBLISH) {
      // Anonymous screens first: /web/auth redirects a logged-in user to /web,
      // so the login card only exists before anything authenticates.
      const anonymous = PUBLISH_SCREENS.filter((entry) => entry.anonymous);
      if (anonymous.length > 0) {
        const anon = await openSession(VIEWPORT);
        for (const entry of anonymous) {
          await anon.goto(PRESETS[entry.preset].path);
          await anon.settle(PRESETS[entry.preset].ready);
          await anon.shot(entry.file, true);
        }
        await anon.context.close();
        sessions.length = 0;
      }

      // Then one authenticated session per viewport. The stills are the whole
      // deliverable here, so no recording is made.
      for (const [label, size] of [['desktop', VIEWPORT], ['mobile', MOBILE_VIEWPORT]]) {
        const target = await openSession(size);
        await target.login();
        for (const entry of PUBLISH_SCREENS.filter((screen) => !screen.anonymous)) {
          // A screen without a mobile counterpart is shot once, not twice.
          if (label === 'mobile' && !entry.mobile) continue;
          await target.goto(PRESETS[entry.preset].path);
          await target.settle(PRESETS[entry.preset].ready);
          await target.shot(label === 'mobile' ? `${entry.file}_mobile` : entry.file, true);
        }
      }
      return;
    }

    const session = await openSession(VIEWPORT);
    const { page, shot, goto, settle, login, helpers } = session;
    const video = videoDir ? page.video() : null;

    if (FLOW) {
      // Flows receive helpers only and never log in themselves, so the runner
      // authenticates before handing over — a flow that needs a logged-out
      // page can simply page.goto() past the session.
      await login();
      const module = await import(pathToFileURL(FLOW).href);
      const flow = module.default;
      if (typeof flow !== 'function') {
        throw new UiCaptureError(EXIT_FLOW, `${FLOW} must export a default async function`);
      }
      log(`running custom flow ${FLOW}`);
      await flow(helpers);
    } else if (preset) {
      // An explicit --url always means an authenticated target, regardless of
      // what the preset's own path would have implied about logging in.
      const needsLogin = TARGET ? true : preset.login;
      if (needsLogin) await login();
      const route = TARGET || preset.path;
      await goto(route);
      // A preset's ready selector describes *its* page; applying it to an
      // arbitrary --url target would just time out on a missing element. shot()'s
      // network-idle and settle wait is the readiness contract for --url.
      await settle(TARGET ? '' : preset.ready);
      // Name the still after the route actually visited, not the preset, so a
      // --url capture is self-describing on disk.
      await shot(TARGET ? routeSlug(TARGET) : PRESET);
    } else {
      await login();
      for (const name of ALL_WALK) {
        await goto(PRESETS[name].path);
        await settle(PRESETS[name].ready);
        await shot(name);
      }
    }

    if (video) {
      // Strict order: grab the handle before close, saveAs only after both
      // page and context are closed. Reversing any of these throws.
      await page.close();
      await session.context.close();
      sessions.length = 0;
      const target = path.join(OUT_DIR, `${ARTIFACT_NAME}.webm`);
      await video.saveAs(target);
      await fs.rm(videoDir, { recursive: true, force: true });
      log(`video ${path.basename(target)}`);
    }
  } finally {
    for (const session of sessions) await session.context.close().catch(() => {});
    await browser.close().catch(() => {});
    if (videoDir) await fs.rm(videoDir, { recursive: true, force: true }).catch(() => {});
  }

  log('done');
}

main().catch((error) => {
  const code = error instanceof UiCaptureError ? error.code : EXIT_FLOW;
  process.stderr.write(`uicapture: ${error.message}\n`);
  process.exit(code);
});