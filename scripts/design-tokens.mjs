#!/usr/bin/env node
// ─────────────────────────────────────────────────────────────────────────────
// scripts/design-tokens.mjs
//
// Projects the canonical design-token source into its artifacts.
//
//   design-system/_tokens.scss       (canonical, light)
//   design-system/_tokens-dark.scss  (canonical, dark)
//         │
//         ├──> design-system/tokens.css       (plain CSS, light :root)
//         ├──> design-system/tokens.dark.css  (plain CSS, dark selector)
//         └──> design-system/tokens.json      (W3C Design Tokens contract)
//
// The generated files are build artifacts — never edit them by hand. Run
// `task design-tokens` after every change to the canonical source.
//
// The generator refuses to write when the source is inconsistent:
//   • a `--token` in the runtime block that has no `$variable`
//   • a `$variable` that never reaches the runtime block
//   • an interpolation `#{$name}` that resolves to nothing
//   • a token declared twice
// ─────────────────────────────────────────────────────────────────────────────

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const DESIGN = path.join(ROOT, 'design-system');

const LIGHT_SCSS = path.join(DESIGN, '_tokens.scss');
const DARK_SCSS = path.join(DESIGN, '_tokens-dark.scss');
const OUT_CSS = path.join(DESIGN, 'tokens.css');
const OUT_DARK_CSS = path.join(DESIGN, 'tokens.dark.css');
const OUT_JSON = path.join(DESIGN, 'tokens.json');

const RUNTIME_MARKER = '// ─────────────────────────────────────────────────────────────────────────────\n// Runtime custom properties';

const fail = (msg) => {
  console.error(`design-tokens: ${msg}`);
  process.exit(1);
};

// ── Parsing ─────────────────────────────────────────────────────────────────

// Section markers look like `// ── Surfaces ──────────────` and name the
// group a following run of variables belongs to.
const GROUP_RE = /^\/\/\s*──\s*(.+?)\s*──/;

// `$name: value;` optionally followed by a `// role` comment.
const VAR_RE = /^\$([\w-]+)\s*:\s*(.+?);\s*(?:\/\/\s*(.*))?$/;

function parseVariables(src) {
  const vars = new Map();
  let group = 'other';
  for (const line of src.split('\n')) {
    const groupMatch = line.match(GROUP_RE);
    if (groupMatch) {
      group = groupMatch[1].replace(/\s*\(.*?\)\s*$/, '').trim();
      continue;
    }
    const varMatch = line.match(VAR_RE);
    if (!varMatch) continue;
    const [, name, value, role] = varMatch;
    if (vars.has(name)) fail(`duplicate $${name}`);
    vars.set(name, { name, value: value.trim(), role: role ? role.trim() : '', group });
  }
  return vars;
}

// Pull the CSS body out of the canonical file: everything from the runtime
// marker down, with `#{$var}` interpolations resolved.
function extractRuntimeBody(src, vars, file) {
  const markerAt = src.indexOf(RUNTIME_MARKER);
  if (markerAt === -1) fail(`${file}: runtime custom properties marker not found`);
  let body = src.slice(markerAt + RUNTIME_MARKER.length);

  // The dark file keeps its tokens in a mixin; unwrap it.
  const mixinMatch = body.match(/@mixin\s+proviant-dark-tokens\s*\{([\s\S]*?)\n\}/);
  if (mixinMatch) body = mixinMatch[1];

  body = body.replace(/#\{\$([\w-]+)\}/g, (_, name) => {
    if (!vars.has(name)) fail(`${file}: interpolation #{$${name}} has no matching variable`);
    return vars.get(name).value;
  });

  const stray = body.match(/#\{\$[\w-]+\}/);
  if (stray) fail(`${file}: unresolved interpolation ${stray[0]}`);

  // SCSS line comments are not valid CSS — the emitted artifacts are plain
  // CSS, so every `// …` becomes a `/* … */` block.
  body = body
    .split('\n')
    .map((line) => {
      const comment = line.match(/^(\s*)\/\/\s?(.*)$/);
      if (!comment) return line;
      // Pure separator rules carry no information — drop them.
      if (/^[─━═=\-_*#·.\s]+$/.test(comment[2])) return null;
      return `${comment[1]}/* ${comment[2]} */`;
    })
    .filter((line) => line !== null)
    .join('\n');

  // Trim blank lines only — a plain trim() would strip the indentation of the
  // first declaration.
  return body.replace(/^\n+/, '').replace(/\s+$/, '');
}

// Every `--token: value;` in the primary block of the runtime body.
//
// The light file ends with a `prefers-reduced-motion` block that re-declares
// three motion tokens with a zero duration. That is a legitimate override, not
// a duplicate, so only the first block is read here.
function extractCustomProperties(body) {
  const blockAt = body.indexOf('{');
  const scope = blockAt === -1 ? body : body.slice(blockAt + 1, body.indexOf('\n}'));

  const props = new Map();
  for (const line of scope.split('\n')) {
    const match = line.match(/^\s*(--[\w-]+)\s*:\s*(.+?);\s*$/);
    if (!match) continue;
    const [, name, value] = match;
    if (props.has(name)) fail(`custom property ${name} declared twice in the runtime block`);
    props.set(name, value.trim());
  }
  return props;
}

// ── Type inference for the JSON contract ────────────────────────────────────

const isColor = (v) => /^#[0-9a-fA-F]{3,8}$/.test(v) || /^rgba?\(/i.test(v);
const isFontFamily = (v) => /"[A-Za-z]/.test(v) && v.includes(',') && !v.includes('px');
// A shadow is a multi-part value that carries lengths; a single `40px` is a
// dimension, `none` is the empty shadow.
const isShadow = (v) => v === 'none' || (v.includes('px') && !/^-?[\d.]+px$/.test(v.trim()));

function inferType(value) {
  if (isColor(value)) return 'color';
  if (value === 'none') return 'shadow';
  if (isFontFamily(value)) return 'fontFamily';
  if (isShadow(value)) return 'shadow';
  if (/ms$/.test(value)) return 'duration';
  if (/px$/.test(value)) return 'dimension';
  if (/^(cubic-bezier|steps)/.test(value)) return 'timingFunction';
  if (/^-?[\d.]+(em|rem|%)?$/.test(value)) return 'number';
  return 'string';
}

// ── JSON contract ───────────────────────────────────────────────────────────

// SCSS source group -> DTCG group. Aliases and status colours are all colours.
function jsonGroup(group) {
  switch (group) {
    case 'Surfaces':
    case 'Ink':
    case 'Lines':
    case 'Accent':
    case 'Vorratsstatus':
    case 'Semantische Aliase':
      return 'color';
    case 'Schatten':
      return 'shadow';
    case 'Typografie':
      return 'typography';
    case 'Spacing':
      return 'spacing';
    case 'Radius':
      return 'radius';
    case 'Motion':
      return 'motion';
    case 'Layout':
      return 'layout';
    default:
      return 'other';
  }
}

// The focus ring belongs next to the shadows it is built from.
const FOCUS_TOKENS = new Set(['focus-ring']);

function buildContract(lightVars) {
  const contract = {
    $schema: 'https://design-tokens.github.io/community-group/format/',
    $description:
      'proviant — Design Tokens (W3C Design Tokens Format). Generated from ' +
      'design-system/_tokens.scss by scripts/design-tokens.mjs — do not edit.',
  };

  for (const entry of lightVars.values()) {
    const group = FOCUS_TOKENS.has(entry.name) ? 'focus' : jsonGroup(entry.group);
    if (!contract[group]) contract[group] = {};
    const token = { $value: entry.value, $type: inferType(entry.value) };
    if (entry.role) token.$description = entry.role;
    contract[group][entry.name] = token;
  }

  return contract;
}

// ── Emit ────────────────────────────────────────────────────────────────────

const BANNER_LIGHT = `/* ─────────────────────────────────────────────────────────────────────────
 * design-system/tokens.css
 *
 * GENERATED FILE — do not edit.
 * Source: design-system/_tokens.scss  (regenerate with \`task design-tokens\`)
 *
 * Crème + Salbei. Schema-Slots: --bg, --surface, --fg, --muted, --border,
 * --accent. Vorratsstatus: --status-fresh, --status-soon, --status-critical,
 * --status-expired, --status-neutral. Textvarianten: --status-*-text,
 * --accent-text.
 * ─────────────────────────────────────────────────────────────────── */

`;

const BANNER_DARK = `/* ─────────────────────────────────────────────────────────────────────────
 * design-system/tokens.dark.css
 *
 * GENERATED FILE — do not edit.
 * Source: design-system/_tokens-dark.scss  (regenerate with \`task design-tokens\`)
 *
 * Opt-in via <html data-bs-theme="dark">, mit Fallback auf
 * prefers-color-scheme: dark. theme.js setzt data-bs-theme immer explizit.
 * ─────────────────────────────────────────────────────────────────── */

`;

function main() {
  const lightSrc = fs.readFileSync(LIGHT_SCSS, 'utf8');
  const darkSrc = fs.readFileSync(DARK_SCSS, 'utf8');

  const lightVars = parseVariables(lightSrc);
  const darkVars = parseVariables(darkSrc);
  if (lightVars.size === 0) fail('no variables parsed from _tokens.scss');

  const lightBody = extractRuntimeBody(lightSrc, lightVars, '_tokens.scss');
  const darkBody = extractRuntimeBody(darkSrc, darkVars, '_tokens-dark.scss');

  const lightProps = extractCustomProperties(lightBody);
  const darkProps = extractCustomProperties(darkBody);

  // The runtime block and the variable list must be a bijection: every
  // variable reaches the browser, every published token has a source value.
  for (const name of lightVars.keys()) {
    if (!lightProps.has(`--${name}`)) fail(`$${name} never reaches the runtime block`);
  }
  for (const name of lightProps.keys()) {
    if (!lightVars.has(name.slice(2))) fail(`${name} has no $variable`);
  }
  for (const name of darkVars.keys()) {
    if (!darkProps.has(`--${name}`)) fail(`dark $${name} never reaches the runtime block`);
  }
  for (const name of darkProps.keys()) {
    if (!darkVars.has(name.slice(2))) fail(`dark ${name} has no $variable`);
  }

  // The dark theme may only override tokens the light theme declares.
  for (const name of darkProps.keys()) {
    if (!lightProps.has(name)) fail(`dark theme overrides ${name}, which the light theme does not declare`);
  }

  const darkCss = [
    '[data-bs-theme="dark"] {',
    darkBody,
    '}',
    '',
    '@media (prefers-color-scheme: dark) {',
    '  :root:not([data-bs-theme="light"]) {',
    darkBody,
    '  }',
    '}',
  ].join('\n');

  const contract = `${JSON.stringify(buildContract(lightVars), null, 2)}\n`;

  const artifacts = [
    [OUT_CSS, `${BANNER_LIGHT}${lightBody}\n`],
    [OUT_DARK_CSS, `${BANNER_DARK}${darkCss}\n`],
    [OUT_JSON, contract],
  ];

  if (process.argv.includes('--check')) {
    const stale = [];
    for (const [file, expected] of artifacts) {
      const actual = fs.existsSync(file) ? fs.readFileSync(file, 'utf8') : null;
      if (actual !== expected) stale.push(path.relative(ROOT, file));
    }
    if (stale.length > 0) {
      fail(`stale artifacts: ${stale.join(', ')} — run \`task design-tokens\``);
    }
    console.log('design-tokens: artifacts are up to date');
    return;
  }

  for (const [file, content] of artifacts) fs.writeFileSync(file, content);

  console.log(
    `design-tokens: ${lightVars.size} light tokens, ${darkVars.size} dark overrides, ` +
      `${lightProps.size} custom properties -> tokens.css, tokens.dark.css, tokens.json`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main();

export { parseVariables, extractRuntimeBody, extractCustomProperties, inferType, ROOT, DESIGN, LIGHT_SCSS, DARK_SCSS, OUT_CSS, OUT_DARK_CSS, OUT_JSON };
