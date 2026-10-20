#!/usr/bin/env node
// ─────────────────────────────────────────────────────────────────────────────
// scripts/check-design-tokens.mjs
//
// Acceptance gate for the design-system migration.
//
//   node scripts/check-design-tokens.mjs [--phase N]
//
// `--phase` gates the checks that only become meaningful once the migration
// has reached that stage. Raise the phase as the migration lands, so the
// checker never fails on work that has not happened yet.
//
//   phase 1  token contract, contrast, manifest counts, spec-sheet wiring
//   phase 2  the compiled app CSS carries the canonical tokens
//   phase 3  no legacy class names, no hardcoded colours in JS,
//            every CSS variable JS reads exists
//
// Exit code 0 = all enabled checks pass.
// ─────────────────────────────────────────────────────────────────────────────

import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import {
  parseVariables,
  extractRuntimeBody,
  extractCustomProperties,
  ROOT,
  DESIGN,
  LIGHT_SCSS,
  DARK_SCSS,
  OUT_JSON,
} from './design-tokens.mjs';

const phaseArg = process.argv.indexOf('--phase');
const PHASE = phaseArg === -1 ? 1 : Number(process.argv[phaseArg + 1]);

// ── Reporting ───────────────────────────────────────────────────────────────

const failures = [];

const fail = (check, message) => failures.push(`${check}: ${message}`);

// ── Colour maths (WCAG 2.1) ────────────────────────────────────────────────

const srgbToLinear = (c) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
const linearToSrgb = (c) => (c <= 0.0031308 ? 12.92 * c : 1.055 * c ** (1 / 2.4) - 0.055);

function parseColor(value) {
  if (typeof value !== 'string') return null;
  const hex = value.trim().match(/^#([0-9a-fA-F]{3,8})$/);
  if (hex) {
    let h = hex[1];
    if (h.length === 3 || h.length === 4) h = [...h].map((c) => c + c).join('');
    return {
      r: parseInt(h.slice(0, 2), 16) / 255,
      g: parseInt(h.slice(2, 4), 16) / 255,
      b: parseInt(h.slice(4, 6), 16) / 255,
      a: h.length === 8 ? parseInt(h.slice(6, 8), 16) / 255 : 1,
    };
  }
  const rgb = value.trim().match(/^rgba?\(([^)]+)\)$/i);
  if (rgb) {
    const parts = rgb[1].split(/[\s,/]+/).filter(Boolean);
    if (parts.length < 3) return null;
    const [r, g, b] = parts.slice(0, 3).map((p) => (p.endsWith('%') ? parseFloat(p) / 100 : parseFloat(p) / 255));
    const a = parts[3] === undefined ? 1 : parseFloat(parts[3]);
    return { r, g, b, a: Number.isNaN(a) ? 1 : a };
  }
  return null;
}

const luminance = ({ r, g, b }) =>
  0.2126 * srgbToLinear(r) + 0.7152 * srgbToLinear(g) + 0.0722 * srgbToLinear(b);

function contrast(a, b) {
  const la = luminance(a);
  const lb = luminance(b);
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

// ── Oklab mixing, matching CSS `color-mix(in oklab, …)` ───────────────────

function toOklab({ r, g, b }) {
  const lr = srgbToLinear(r);
  const lg = srgbToLinear(g);
  const lb = srgbToLinear(b);
  const l = Math.cbrt(0.4122214708 * lr + 0.5363325363 * lg + 0.0514459929 * lb);
  const m = Math.cbrt(0.2119034982 * lr + 0.6806995451 * lg + 0.1073969566 * lb);
  const s = Math.cbrt(0.0883024619 * lr + 0.2817188376 * lg + 0.6299787005 * lb);
  return {
    L: 0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s,
    a: 1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s,
    b: 0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s,
  };
}

function fromOklab({ L, a, b }) {
  const l = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3;
  const m = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3;
  const s = (L - 0.0894841775 * a - 1.291485548 * b) ** 3;
  const lr = 4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s;
  const lg = -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s;
  const lb = -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s;
  const clamp = (v) => Math.max(0, Math.min(1, v));
  return { r: clamp(linearToSrgb(clamp(lr))), g: clamp(linearToSrgb(clamp(lg))), b: clamp(linearToSrgb(clamp(lb))), a: 1 };
}

// `color-mix(in oklab, a p%, b)` — p is the share of `a`.
function mixOklab(a, b, p) {
  const A = toOklab(a);
  const B = toOklab(b);
  const cA = Math.hypot(A.a, A.b);
  const cB = Math.hypot(B.a, B.b);
  const hA = Math.atan2(A.b, A.a);
  const hB = Math.atan2(B.b, B.a);
  // Interpolate the hue along the shorter arc.
  let d = hA - hB;
  if (d > Math.PI) d -= 2 * Math.PI;
  if (d < -Math.PI) d += 2 * Math.PI;
  const h = hB + d * p;
  const c = cB + (cA - cB) * p;
  const L = B.L + (A.L - B.L) * p;
  return fromOklab({ L, a: Math.cos(h) * c, b: Math.sin(h) * c });
}

// ── Load the canonical source ──────────────────────────────────────────────

const lightSrc = fs.readFileSync(LIGHT_SCSS, 'utf8');
const darkSrc = fs.readFileSync(DARK_SCSS, 'utf8');
const lightVars = parseVariables(lightSrc);
const darkVars = parseVariables(darkSrc);
const lightBody = extractRuntimeBody(lightSrc, lightVars, '_tokens.scss');
const darkBody = extractRuntimeBody(darkSrc, darkVars, '_tokens.dark.scss');
const lightProps = extractCustomProperties(lightBody);
const darkProps = extractCustomProperties(darkBody);

// ── Check 1: the generated artifacts are current ────────────────────────────

function checkArtifactsCurrent() {
  const result = spawnSync(process.execPath, [path.join(ROOT, 'scripts', 'design-tokens.mjs'), '--check'], {
    encoding: 'utf8',
  });
  if (result.status !== 0) {
    fail('artifacts', (result.stderr || result.stdout || 'design-tokens --check failed').trim());
  }
}

// ── Check 2: tokens.json mirrors the canonical source ───────────────────────

function checkJsonContract() {
  if (!fs.existsSync(OUT_JSON)) {
    fail('json', 'tokens.json is missing');
    return;
  }
  const contract = JSON.parse(fs.readFileSync(OUT_JSON, 'utf8'));
  const flat = {};
  for (const [group, tokens] of Object.entries(contract)) {
    if (group.startsWith('$')) continue;
    for (const [name, token] of Object.entries(tokens)) flat[name] = token.$value;
  }
  for (const [name, entry] of lightVars) {
    if (!(name in flat)) {
      fail('json', `token $${name} is missing from tokens.json`);
      continue;
    }
    if (flat[name] !== entry.value) {
      fail('json', `token $${name} is ${entry.value} in the source but ${flat[name]} in tokens.json`);
    }
  }
  for (const name of Object.keys(flat)) {
    if (!lightVars.has(name)) fail('json', `tokens.json declares ${name}, which the source does not`);
  }
}

// ── Check 3: contrast targets ──────────────────────────────────────────────

const TEXT = 4.5;
const NON_TEXT = 3;

function checkContrast(label, fg, bg, target) {
  const a = typeof fg === 'string' ? parseColor(fg) : fg;
  const b = typeof bg === 'string' ? parseColor(bg) : bg;
  if (!a || !b) {
    fail('contrast', `${label}: cannot parse ${fg} on ${bg}`);
    return null;
  }
  const ratio = contrast(a, b);
  if (ratio < target) {
    fail('contrast', `${label}: ${ratio.toFixed(2)}:1 on ${bg} — below ${target}:1`);
  }
  return ratio;
}

function checkThemeContrast(theme, props, label) {
  const bg = props.get('--bg');
  const surface = props.get('--surface');
  const accent = props.get('--accent');
  const accentOn = props.get('--accent-on');
  const accentText = props.get('--accent-text');
  const accentSoft = props.get('--accent-soft');

  for (const ink of ['--fg', '--fg-2', '--muted', '--meta']) {
    checkContrast(`${label} ${ink} on --bg`, props.get(ink), bg, TEXT);
  }
  // Tertiary text also sits on cards.
  checkContrast(`${label} --meta on --surface`, props.get('--meta'), surface, TEXT);
  checkContrast(`${label} --accent-on on --accent`, accentOn, accent, TEXT);
  checkContrast(`${label} --accent-text on --bg`, accentText, bg, TEXT);
  checkContrast(`${label} --accent-text on --accent-soft`, accentText, accentSoft, TEXT);
  checkContrast(`${label} --accent on --bg (non-text)`, accent, bg, NON_TEXT);

  for (const state of ['fresh', 'soon', 'critical', 'expired', 'neutral']) {
    const base = props.get(`--status-${state}`);
    const text = props.get(`--status-${state}-text`);
    if (!base || !text) {
      fail('contrast', `${label} status "${state}" is missing a base or text token`);
      continue;
    }
    // The base colour paints icons, dots, bars and borders — a graphical
    // state indicator needs 3:1.
    checkContrast(`${label} --status-${state} on --bg (indicator)`, base, bg, NON_TEXT);
    // The text variant paints labels on a page, on a card and on its own chip.
    checkContrast(`${label} --status-${state}-text on --bg`, text, bg, TEXT);
    checkContrast(`${label} --status-${state}-text on --surface`, text, surface, TEXT);
    const chipBg = mixOklab(parseColor(base), parseColor(surface), 0.18);
    checkContrast(`${label} --status-${state}-text on its chip`, text, chipBg, TEXT);
  }
}

// ── Check 4: the manifest counts match reality ──────────────────────────────

function readManifest() {
  const manifestPath = path.join(DESIGN, 'components.manifest.json');
  if (!fs.existsSync(manifestPath)) {
    fail('manifest', 'components.manifest.json is missing');
    return null;
  }
  return JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
}

function checkManifestCounts() {
  const manifest = readManifest();
  if (!manifest) return;

  const declared = lightProps.size;
  if (manifest.declaredTokens !== declared) {
    fail('manifest', `declaredTokens is ${manifest.declaredTokens}, tokens.css declares ${declared}`);
  }

  // selectors = sum of the selector entries across all groups.
  let selectors = 0;
  const classes = new Set();
  for (const group of manifest.groups ?? []) {
    const entries = group.selectors ?? [];
    selectors += entries.length;
    for (const entry of entries) {
      for (const part of entry.split(',')) {
        const cls = part.trim().split(/\s+/)[0] ?? '';
        if (cls.startsWith('.')) classes.add(cls);
      }
    }
  }
  // classes = distinct class names from the groups plus the utilities.
  for (const [key, value] of Object.entries(manifest.utilities ?? {})) {
    if (key.startsWith('_')) continue;
    if (typeof value !== 'string') continue;
    for (const token of value.split(/[\s,]+/)) {
      if (token.startsWith('.')) classes.add(token);
    }
  }
  if (manifest.selectors !== selectors) {
    fail('manifest', `selectors is ${manifest.selectors}, the groups declare ${selectors}`);
  }
  if (manifest.classes !== classes.size) {
    fail('manifest', `classes is ${manifest.classes}, the design system defines ${classes.size}`);
  }
}

// The sheet-derived counts only become meaningful once the spec sheets run on
// the canonical tokens (PR 6).
function checkManifestSheetCounts() {
  const manifest = readManifest();
  if (!manifest) return;

  const screensDir = path.join(DESIGN, 'screens');
  const sheets = fs.existsSync(screensDir)
    ? fs.readdirSync(screensDir).filter((f) => f.endsWith('.html'))
    : [];

  // Custom properties referenced by the spec sheets.
  const referenced = new Set();
  for (const sheet of sheets) {
    const src = fs.readFileSync(path.join(screensDir, sheet), 'utf8');
    for (const m of src.matchAll(/var\((--[\w-]+)/g)) referenced.add(m[1]);
  }
  if (manifest.referencedTokens !== referenced.size) {
    fail('manifest', `referencedTokens is ${manifest.referencedTokens}, the spec sheets reference ${referenced.size}`);
  }
  for (const name of referenced) {
    if (!lightProps.has(name) && !darkProps.has(name)) {
      fail('manifest', `the spec sheets reference ${name}, which no theme declares`);
    }
  }
}

// ── Check 5: the spec sheets point at the canonical tokens ─────────────────

function checkSpecSheets() {
  const screensDir = path.join(DESIGN, 'screens');
  if (!fs.existsSync(screensDir)) return;
  for (const sheet of fs.readdirSync(screensDir).filter((f) => f.endsWith('.html'))) {
    const src = fs.readFileSync(path.join(screensDir, sheet), 'utf8');
    if (/\/\*\s*:root\s*\{/.test(src) || /^\s*:root\s*\{/m.test(src)) {
      fail('spec-sheets', `${sheet} declares its own :root block — it must use the canonical tokens`);
    }
    if (!/tokens\.css/.test(src)) {
      fail('spec-sheets', `${sheet} does not link tokens.css`);
    }
  }
}

// ── Check 6 (phase 2): the compiled app CSS carries the canonical tokens ────

function checkCompiledCss() {
  const cssPath = path.join(ROOT, 'src', 'assets', 'css', 'main.css');
  if (!fs.existsSync(cssPath)) {
    fail('compiled-css', 'src/assets/css/main.css is missing — run `task css`');
    return;
  }
  const css = fs.readFileSync(cssPath, 'utf8');
  for (const [name, value] of lightProps) {
    const pattern = new RegExp(`${name.replace(/[-]/g, '\\-')}\\s*:\\s*${value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\s*;`);
    if (!pattern.test(css)) {
      fail('compiled-css', `${name} is not compiled into main.css with the value ${value}`);
    }
  }
}

// ── Check 7 (phase 3): the legacy vocabulary is gone ───────────────────────

const LEGACY_CLASSES = [
  'btn-proviant-primary', 'btn-proviant-secondary', 'btn-proviant-danger',
  'btn-proviant-warning', 'btn-ghost', 'badge-status', 'badge-fresh',
  'badge-soon', 'badge-critical', 'badge-expired', 'badge-nodate',
  'badge-lowstock', 'badge-opened', 'field-input', 'field-label',
  'field-error', 'input-hint', 'text-primary-custom', 'text-secondary-custom',
  'text-tertiary', 'metric-tile', 'metric-label', 'metric-value',
  'skeleton-block', 'pv-sidebar', 'pv-nav-item', 'login-root', 'login-brand',
  'login-form-panel', 'sl-list-row', 'modal-icon', 'btn-modal-close', 'pv-toast',
];

const LEGACY_DYNAMIC = ['badge-{{', 'btn-proviant-{{', 'status-{{'];

function sourceFiles(dirs, exts) {
  const out = [];
  const walk = (dir) => {
    if (!fs.existsSync(dir)) return;
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) walk(full);
      else if (exts.some((e) => entry.name.endsWith(e))) out.push(full);
    }
  };
  for (const dir of dirs) walk(path.join(ROOT, dir));
  return out;
}

function checkLegacyClasses() {
  const files = [
    ...sourceFiles(['src/templates/web'], ['.tmpl']),
    ...sourceFiles(['src/assets/js'], ['.js']),
    ...sourceFiles(['tests/js'], ['.js']),
  ];
  for (const file of files) {
    const src = fs.readFileSync(file, 'utf8');
    const rel = path.relative(ROOT, file);
    for (const cls of LEGACY_CLASSES) {
      const pattern = new RegExp(`(^|[\\s"'\`])${cls.replace(/[-]/g, '\\-')}([\\s"'\`]|\\b)`, 'm');
      if (pattern.test(src)) fail('legacy-class', `${rel} still uses .${cls}`);
    }
    for (const marker of LEGACY_DYNAMIC) {
      if (src.includes(marker)) fail('legacy-class', `${rel} still interpolates ${marker}`);
    }
  }
}

// ── Check 8 (phase 3): no hardcoded colours in JS ──────────────────────────

function checkHardcodedColorsInJs() {
  const files = sourceFiles(['src/assets/js'], ['.js']).filter((f) => !/bootstrap\.bundle|chart\.umd|html5-qrcode/.test(f));
  for (const file of files) {
    const src = fs.readFileSync(file, 'utf8');
    const rel = path.relative(ROOT, file);
    for (const m of src.matchAll(/oklch\([^)]*\)/g)) {
      fail('js-color', `${rel} hardcodes ${m[0]} — use a design token`);
    }
    for (const m of src.matchAll(/#[0-9a-fA-F]{6}\b/g)) {
      fail('js-color', `${rel} hardcodes ${m[0]} — use a design token`);
    }
  }
}

// ── Check 9 (phase 3): every CSS variable JS reads exists ──────────────────

function checkJsTokenReferences() {
  const known = new Set([...lightProps.keys(), ...darkProps.keys()]);
  const files = [...sourceFiles(['src/assets/js'], ['.js']), ...sourceFiles(['tests/js'], ['.js'])];
  for (const file of files) {
    const src = fs.readFileSync(file, 'utf8');
    const rel = path.relative(ROOT, file);
    for (const m of src.matchAll(/getPropertyValue\(\s*['"](--[\w-]+)['"]/g)) {
      if (!known.has(m[1])) fail('js-token', `${rel} reads ${m[1]}, which no theme declares`);
    }
  }
}

// ── Run ─────────────────────────────────────────────────────────────────────

checkArtifactsCurrent();
checkJsonContract();
checkThemeContrast('light', lightProps, 'light');
checkThemeContrast('dark', darkProps, 'dark');
checkManifestCounts();

if (PHASE >= 2) checkCompiledCss();
if (PHASE >= 3) {
  checkLegacyClasses();
  checkHardcodedColorsInJs();
  checkJsTokenReferences();
}
if (PHASE >= 6) {
  checkSpecSheets();
  checkManifestSheetCounts();
}

if (failures.length > 0) {
  console.error(`\ndesign-check: ${failures.length} problem(s) at phase ${PHASE}`);
  for (const message of failures) console.error(`  ✗ ${message}`);
  process.exit(1);
}

console.log(`design-check: all checks passed (phase ${PHASE})`);
