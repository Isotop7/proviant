#!/usr/bin/env node
// Host orchestrator for the ui-capture skill.
//
// Owns everything that is not a browser: preflight, Go build, demo seed,
// temp config, server lifecycle, container launch, ffmpeg transcodes, teardown.
// Deliberately split from browser-capture.mjs so the browser stage runs
// unchanged under the Playwright container's Node or under host Node.
//
//   node .kilo/skills/ui-capture/scripts/uicapture.mjs --preset dashboard
//
// Exit codes (see SKILL.md):
//   0  success, or ffmpeg missing with --mp4/--gif (warn, keep webm and stills)
//   1  bad usage
//   2  port busy                 3  compiled CSS / config source missing
//   4  Chromium missing          5  server never became ready
//   6  login rejected            7  flow selector/URL timeout
//   8  no container runtime      9  Playwright image not present locally
//  10  chromium launch failure

import { spawn, spawnSync } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import fsSync from 'node:fs';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = path.resolve(SCRIPT_DIR, '..', '..', '..', '..');
const SKILL_DIR = path.resolve(SCRIPT_DIR, '..');
const BROWSER_SCRIPT = path.join(SCRIPT_DIR, 'browser-capture.mjs');
const PRESETS_MODULE = path.join(SCRIPT_DIR, 'presets.mjs');
const IMAGE = 'mcr.microsoft.com/playwright';

// playwright lives in the skill's own node_modules, not the repo's. As a repo
// devDependency it was installed by every Docker build and every CI job for a
// tool no build step invokes, and it dragged playwright-core's ~14 MB along.
// .kilo/.gitignore keeps this directory out of git.
const PLAYWRIGHT_VERSION = '1.63.0';
const SKILL_MODULES = path.join(SKILL_DIR, 'node_modules');
const PLAYWRIGHT_PKG = path.join(SKILL_MODULES, 'playwright', 'package.json');

const VIEWPORTS = { desktop: '1440x900', mobile: '390x844' };

class UiCaptureError extends Error {
  constructor(code, message) {
    super(message);
    this.code = code;
  }
}

const out = (line) => process.stdout.write(`${line}\n`);
const err = (line) => process.stderr.write(`${line}\n`);
const step = (line) => err(`[uicapture] ${line}`);

// ── args ────────────────────────────────────────────────────────────────────

const { presetNames } = await import(pathToFileURL(PRESETS_MODULE).href);

// The preset names come from the module the browser stage actually shoots, so
// --help cannot list a preset that no longer exists or omit one that does.
// Wrapped to keep the flag column aligned with the rest of the usage block.
function wrapPresets(names, label = '--preset <name>', continuation = 22, width = 78) {
  const lines = [];
  let line = `  ${label}`.padEnd(continuation);
  for (const name of names) {
    const chunk = ` ${name} |`;
    if (line.length + chunk.length > width && line.trim() !== `  ${label}`) {
      lines.push(line.trimEnd());
      line = ' '.repeat(continuation);
    }
    line += chunk;
  }
  lines.push(line.trimEnd().replace(/\|$/, '').trimEnd());
  return lines.join('\n');
}

const USAGE = `usage: uicapture.mjs [options]

${wrapPresets(['dashboard (default)', 'all', ...presetNames()])}
  --url <path>        capture one authenticated route instead of the preset's own
  --script <file>     run a custom flow module instead of a preset walk
  --viewport <v>      desktop (default) | mobile | WxH
  --out <dir>         output directory (default .cache/ui-captures/<preset>-<stamp>)
  --mp4               also transcode the webm to mp4 (needs host ffmpeg with an
                      H.264 encoder: libx264, libopenh264 or h264_vaapi)
  --gif               also transcode the webm to gif (needs host ffmpeg)
  --full-page         stills span the whole document; fixed chrome is re-anchored
  --publish           regenerate screenshots/ and the README gallery, then exit
  --no-video          stills only, skip recording
  --browser <mode>    container (default) | local
  --port <n>          server port, 0 picks an ephemeral one (default 5114)
  --keep-server       leave the temp server dir in place and print its path
  --timeout <ms>      server readiness + browser step timeout (default 45000)
  --help              this text`;

function parseArgs(argv) {
  const opts = {
    preset: 'dashboard',
    url: '',
    script: '',
    viewport: 'desktop',
    out: '',
    mp4: false,
    gif: false,
    fullPage: false,
    publish: false,
    video: true,
    browser: 'container',
    port: 5114,
    keepServer: false,
    timeout: 45000,
    help: false,
  };

  const usage = (message) => new UiCaptureError(1, `${message}\n${USAGE}`);
  const list = argv.slice();
  while (list.length > 0) {
    const arg = list.shift();
    const eq = arg.indexOf('=');
    const flag = eq === -1 ? arg : arg.slice(0, eq);
    const inline = eq === -1 ? null : arg.slice(eq + 1);
    const value = (name) => {
      if (inline !== null) return inline;
      if (list.length === 0) throw usage(`${name} needs a value`);
      return list.shift();
    };

    switch (flag) {
      case '--preset': opts.preset = value(flag); break;
      case '--url': opts.url = value(flag); break;
      case '--script': opts.script = value(flag); break;
      case '--viewport': opts.viewport = value(flag); break;
      case '--out': opts.out = value(flag); break;
      case '--browser': opts.browser = value(flag); break;
      case '--port': opts.port = Number.parseInt(value(flag), 10); break;
      case '--timeout': opts.timeout = Number.parseInt(value(flag), 10); break;
      case '--mp4': opts.mp4 = true; break;
      case '--gif': opts.gif = true; break;
      case '--full-page': opts.fullPage = true; break;
      case '--publish': opts.publish = true; break;
      case '--no-video': opts.video = false; break;
      case '--keep-server': opts.keepServer = true; break;
      case '--help': case '-h': opts.help = true; break;
      default: throw usage(`unknown option "${arg}"`);
    }
  }

  if (!VIEWPORTS[opts.viewport] && !/^\d+x\d+$/.test(opts.viewport)) {
    throw usage(`--viewport must be desktop, mobile or WxH, got "${opts.viewport}"`);
  }
  opts.viewport = VIEWPORTS[opts.viewport] || opts.viewport;
  if (opts.browser !== 'container' && opts.browser !== 'local') {
    throw usage(`--browser must be container or local, got "${opts.browser}"`);
  }
  if (!Number.isInteger(opts.port) || opts.port < 0 || opts.port > 65535) {
    throw usage(`--port must be 0-65535, got "${opts.port}"`);
  }
  if (opts.preset === 'all' && opts.url) {
    throw usage('--url cannot be combined with --preset all; use a single preset');
  }
  if (opts.script && opts.url) {
    throw usage('--url and --script both replace the preset target; pick one');
  }
  if (!opts.video && (opts.mp4 || opts.gif)) {
    throw usage('--mp4/--gif need a recording; drop --no-video');
  }
  if (opts.publish && (opts.script || opts.url || opts.fullPage || opts.mp4 || opts.gif)) {
    throw usage('--publish runs a fixed screen set; it takes no --script/--url/--full-page/--mp4/--gif');
  }
  if (opts.publish && opts.preset !== 'dashboard') {
    throw usage('--publish ignores --preset; it always shoots the README screen set');
  }
  return opts;
}

// ── small helpers ───────────────────────────────────────────────────────────

function run(command, args, options = {}) {
  const { quiet, ...rest } = options;
  return new Promise((resolve) => {
    const child = spawn(command, args, {
      stdio: ['ignore', quiet ? 'ignore' : 'inherit', quiet ? 'ignore' : 'inherit'],
      ...rest,
    });
    child.on('error', () => resolve({ code: 127 }));
    child.on('close', (code) => resolve({ code: code === null ? 1 : code }));
  });
}

function runSync(command, args, options = {}) {
  return spawnSync(command, args, { stdio: 'inherit', ...options });
}

function which(binary) {
  for (const dir of (process.env.PATH || '').split(path.delimiter).filter(Boolean)) {
    const candidate = path.join(dir, binary);
    try {
      fsSync.accessSync(candidate, fsSync.constants.X_OK);
      if (fsSync.statSync(candidate).isFile()) return candidate;
    } catch { /* keep looking */ }
  }
  return '';
}

function firstExisting(candidates) {
  return candidates.find((candidate) => fsSync.existsSync(candidate)) || '';
}

function stamp(date = new Date()) {
  const p = (n) => String(n).padStart(2, '0');
  return `${date.getFullYear()}${p(date.getMonth() + 1)}${p(date.getDate())}-${p(date.getHours())}${p(date.getMinutes())}${p(date.getSeconds())}`;
}

async function freePort() {
  return new Promise((resolve, reject) => {
    const probe = net.createServer();
    probe.on('error', reject);
    probe.listen(0, '127.0.0.1', () => {
      const { port } = probe.address();
      probe.close(() => resolve(port));
    });
  });
}

function portIsFree(port) {
  return new Promise((resolve) => {
    const probe = net.createServer();
    probe.once('error', () => resolve(false));
    probe.once('listening', () => probe.close(() => resolve(true)));
    probe.listen(port, '127.0.0.1');
  });
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

// ── preflight ───────────────────────────────────────────────────────────────

// playwright is a skill-local dependency, so the skill owns installing it.
// --ignore-scripts matters: playwright's install hook downloads browser
// binaries, and in container mode those live in the image at /ms-playwright.
// Only the npm package is needed, and the image ships no browsers for the host.
function installPlaywright() {
  if (fsSync.existsSync(PLAYWRIGHT_PKG)) {
    const installed = JSON.parse(fsSync.readFileSync(PLAYWRIGHT_PKG, 'utf8')).version;
    if (installed === PLAYWRIGHT_VERSION) return installed;
  }

  step(`installing playwright@${PLAYWRIGHT_VERSION} into .kilo/skills/ui-capture/node_modules`);
  const npm = runSync('npm', [
    'install',
    '--prefix', SKILL_DIR,
    '--no-save',
    '--no-audit',
    '--no-fund',
    '--ignore-scripts',
    `playwright@${PLAYWRIGHT_VERSION}`,
  ]);
  if (npm.code !== 0 || !fsSync.existsSync(PLAYWRIGHT_PKG)) {
    throw new UiCaptureError(
      3,
      `could not install playwright@${PLAYWRIGHT_VERSION}.\n` +
        `  Run it by hand:  npm install --prefix .kilo/skills/ui-capture --no-save --ignore-scripts playwright@${PLAYWRIGHT_VERSION}`,
    );
  }
  return JSON.parse(fsSync.readFileSync(PLAYWRIGHT_PKG, 'utf8')).version;
}

function preflight(opts) {
  // 3 — assets.go:5 is a go:embed over css/, so without a compiled main.css the
  // Go build cannot even compile. Say "task css" instead of surfacing that.
  const css = path.join(REPO_ROOT, 'src', 'assets', 'css', 'main.css');
  if (!fsSync.existsSync(css)) {
    throw new UiCaptureError(
      3,
      `${css} is missing.\n  src/assets/assets.go:5 embeds the css/ directory, so the Go build\n` +
        '  cannot compile. Run `task css` first.',
    );
  }

  // 3 — the config is copied verbatim into the temp dir and overridden by
  // PROVIANT_* env, so the tracked template is a valid stand-in on a fresh
  // clone. proviant.go:143 panics when no config file resolves.
  const configSource = firstExisting([
    path.join(REPO_ROOT, 'src', 'config.yaml'),
    path.join(REPO_ROOT, 'src', 'config.yaml.sqlite.tmpl'),
  ]);
  if (!configSource) {
    throw new UiCaptureError(
      3,
      'No config source found: expected src/config.yaml or src/config.yaml.sqlite.tmpl.\n' +
        '  Both are gitignored, so a fresh clone has neither. Restore one, e.g.\n' +
        '  `cp src/config.yaml.sqlite.tmpl src/config.yaml`.',
    );
  }

  // 3/10 — the image tag is derived from the installed package so package and
  // image cannot drift (playwright-core refuses a mismatched browser revision).
  const version = installPlaywright();

  const info = { configSource, version, image: `${IMAGE}:v${version}-noble` };
  if (opts.browser === 'local') return info;

  // 8 — mirrors Taskfile.yml:5 (command -v podman || command -v docker).
  const runtime = which('podman') || which('docker');
  if (!runtime) {
    throw new UiCaptureError(
      8,
      'Neither podman nor docker is on PATH.\n' +
        '  Install podman, or re-run with `--browser local` (see references/fedora-setup.md).',
    );
  }
  info.runtime = path.basename(runtime);

  // 9 — never pull implicitly; a 2 GB download mid-run is unacceptable.
  if (spawnSync(info.runtime, ['image', 'exists', info.image], { stdio: 'ignore' }).status !== 0) {
    throw new UiCaptureError(
      9,
      `Image ${info.image} is not present locally.\n  Pull it once, deliberately:\n\n` +
        `    ${info.runtime} pull ${info.image}\n`,
    );
  }
  return info;
}

// ── server ──────────────────────────────────────────────────────────────────

// Viper keys are the literal source names (baseURL, filepath, imageCacheEnabled);
// the env prefix is PROVIANT and lookups are uppercased.
function serverEnv(port) {
  return {
    ...process.env,
    PROVIANT_SERVER_PORT: String(port),
    PROVIANT_SERVER_BASEURL: `http://127.0.0.1:${port}`,
    PROVIANT_SERVER_DEMO_MODE: 'false',
    // Loopback only. The server binds every interface when this is empty, which
    // would expose the demo instance to the whole network for the run's
    // duration (configuration.go ServerConfiguration.Host, proviant.go).
    PROVIANT_SERVER_HOST: '127.0.0.1',
    // The JWT signing key is this value verbatim (middleware.go JWTMiddleware),
    // so a committed literal would let anyone mint tokens for the demo server.
    // A fresh random secret per run keeps the demo credentials the only way in.
    PROVIANT_SERVER_AUTHENTICATION_TOKENPASSWORD: crypto.randomBytes(32).toString('hex'),
    PROVIANT_DATABASE_SQLITE_FILEPATH: 'data/demo_seed.db',
    // proviant.go:342 panics on an invalid SMTP/ntfy/telegram block, and this
    // host has no tesseract for OCR (proviant.go:361).
    PROVIANT_NOTIFICATION_ENABLED: 'false',
    PROVIANT_OCR_ENABLED: 'false',
    PROVIANT_OPENFOODFACTS_IMAGECACHEENABLED: 'false',
    PROVIANT_OPENFOODFACTS_CACHEENABLED: 'false',
  };
}

function spawnServer(binary, serverDir, env) {
  const logFile = path.join(serverDir, 'stderr.log');
  const stream = fsSync.createWriteStream(logFile);
  const child = spawn(binary, [], { cwd: serverDir, env, stdio: ['ignore', 'pipe', 'pipe'] });
  child.stdout.pipe(stream);
  child.stderr.pipe(stream);
  return { child, logFile, stream };
}

async function waitForServer(child, baseURL, logFile, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  let exited = null;
  child.once('exit', (code, signal) => { exited = { code, signal }; });

  while (Date.now() < deadline) {
    if (exited) break;
    const reachable = await new Promise((resolve) => {
      globalThis
        .fetch(`${baseURL}/web/auth`, { redirect: 'manual' })
        .then(() => resolve(true))
        .catch(() => resolve(false));
    });
    if (reachable) return;
    await sleep(250);
  }

  let tail = '';
  try {
    tail = (await fs.readFile(logFile, 'utf8'))
      .split('\n')
      .filter((line) => line.trim() !== '')
      .slice(-40)
      .join('\n');
  } catch { /* nothing written yet */ }

  throw new UiCaptureError(
    5,
    `Server never became ready at ${baseURL} within ${timeoutMs} ms` +
    (exited ? ` (process exited: code=${exited.code} signal=${exited.signal})` : '') +
    `.\n  Log: ${logFile}\n\n${tail}\n`,
  );
}

async function stopServer(handle) {
  if (!handle) return;
  if (handle.child.exitCode === null && handle.child.signalCode === null) {
    handle.child.kill('SIGTERM');
    await new Promise((resolve) => {
      const timer = setTimeout(() => {
        handle.child.kill('SIGKILL');
        resolve();
      }, 3000);
      handle.child.once('exit', () => { clearTimeout(timer); resolve(); });
    });
  }
  handle.stream.end();
}

// ── browser stage ───────────────────────────────────────────────────────────

// The container shares the host loopback via --network=host, so it reaches the
// host-run server with no port publishing. The skill directory is mounted
// because the image ships the browsers at /ms-playwright but no playwright npm
// package.
function artifactName(opts) {
  return opts.script ? path.basename(opts.script).replace(/\.mjs$/, '') : opts.preset;
}

function browserEnv(opts, outDir) {
  return {
    BASE_URL: `http://127.0.0.1:${opts.port}`,
    OUT_DIR: outDir,
    VIEWPORT: opts.viewport,
    CLIP: opts.fullPage ? '0' : '1',
    VIDEO: opts.video ? '1' : '0',
    PRESET: opts.preset,
    // A flow run has no preset to name the recording after, so it borrows the
    // script's filename instead of inheriting the --preset default.
    ARTIFACT_NAME: artifactName(opts),
    TIMEOUT: String(opts.timeout),
    PUBLISH: opts.publish ? '1' : '0',
    // The README gallery is described in exactly one place (PUBLISHED_SCREENS).
    // The browser stage shoots that list verbatim, so a screen added here can
    // never be captured but silently dropped by publishScreens.
    PUBLISH_SCREENS: JSON.stringify(PUBLISHED_SCREENS),
    ...(opts.url ? { TARGET: opts.url } : {}),
    ...(opts.script ? { FLOW: opts.script } : {}),
  };
}

// Mount only what the browser stage actually reads. A whole-repo `-v repo:/work:Z`
// would relabel every tracked file to container_file_t as a side effect; the
// subtree below is gitignored, and keeping the mount narrow keeps the SELinux
// relabel walk cheap. It also carries the skill's own node_modules, which is
// where playwright lives.
//
// Everything is mounted read-only except the output directory. The browser
// renders user-supplied product image URLs and Chromium runs as container root,
// so a writable mount would hand a renderer compromise a foothold in the host's
// build pipeline.
const FIXED_MOUNTS = ['.kilo'];

function planMount(dir) {
  if (dir === path.parse(dir).root) {
    throw new UiCaptureError(1, `refusing to mount ${dir} into the container; use an --out directory below the repo or a normal path`);
  }
  const rel = path.relative(REPO_ROOT, dir);
  if (rel === '') return { hostPath: dir, mountPoint: '/work' };
  if (rel.startsWith('..')) return { hostPath: dir, mountPoint: '/work/.out' };
  let top = dir;
  for (;;) {
    const parent = path.dirname(top);
    if (parent === REPO_ROOT || parent === top) break;
    top = parent;
  }
  return { hostPath: top, mountPoint: path.posix.join('/work', path.relative(REPO_ROOT, top)) };
}

function containerPath(dir) {
  const { hostPath, mountPoint } = planMount(dir);
  return path.posix.join(mountPoint, path.relative(hostPath, dir));
}

// The container runs unprivileged so Chromium can use its own seccomp sandbox.
// Chromium refuses to start as root without --no-sandbox, and Playwright's
// default (chromiumSandbox: false) turns that refusal into an unsandboxed
// renderer — which, rendering user-supplied image URLs, is exactly the process
// that must not have a writable host mount.
//
// Rootless podman and rootful docker disagree on how to become the invoking
// user, so the flag is picked per runtime. SYS_CHROOT is the one capability the
// Chromium zygote needs for its chroot step; everything else is dropped.
function hardeningArgs(info) {
  const user = `${process.getuid?.() ?? 0}:${process.getgid?.() ?? 0}`;
  const asHostUser = path.basename(info.runtime) === 'docker' ? ['--user', user] : ['--userns=keep-id'];
  return [
    '--cap-drop=ALL',
    '--cap-add=SYS_CHROOT',
    '--security-opt=no-new-privileges',
    ...asHostUser,
  ];
}

function containerArgs(info, opts, outDir) {
  const env = browserEnv(opts, outDir);
  const mounts = new Map(
    FIXED_MOUNTS.map((entry) => {
      const hostPath = path.join(REPO_ROOT, entry);
      return [hostPath, { mountPoint: path.posix.join('/work', entry), readOnly: true }];
    }),
  );
  const addMount = (dir, readOnly) => {
    const plan = planMount(dir);
    const existing = mounts.get(plan.hostPath);
    if (!existing) mounts.set(plan.hostPath, { mountPoint: plan.mountPoint, readOnly });
    else if (!readOnly) existing.readOnly = false;
    return containerPath(dir);
  };

  env.OUT_DIR = addMount(outDir, false);
  if (env.FLOW) env.FLOW = addMount(env.FLOW, true);

  // Host networking, not a published port: the capture server binds loopback
  // only (serverEnv), which a bridge-network container cannot reach. This is
  // also what --browser local does, since it drives the same server from a
  // host browser, so it grants no reach the local mode does not already have.
  const args = ['run', '--rm', '--network=host', '-w', '/work', ...hardeningArgs(info)];
  for (const [hostPath, { mountPoint, readOnly }] of mounts) {
    args.push('-v', `${hostPath}:${mountPoint}:Z${readOnly ? ',ro' : ''}`);
  }
  for (const [key, value] of Object.entries(env)) args.push('-e', `${key}=${value}`);
  args.push(info.image, 'node', '/work/.kilo/skills/ui-capture/scripts/browser-capture.mjs');
  return args;
}

async function runBrowser(info, opts, outDir) {
  if (opts.browser === 'local') {
    step('browser local (host node, host chromium)');
    const result = await run('node', [BROWSER_SCRIPT], {
      cwd: REPO_ROOT,
      env: { ...process.env, ...browserEnv(opts, outDir) },
    });
    return result.code;
  }

  step(`browser container (${info.image})`);
  const result = await run(info.runtime, containerArgs(info, opts, outDir), { cwd: REPO_ROOT });

  // Both runtimes are launched as the invoking user (hardeningArgs), so
  // artifacts come out owned by you and no chown dance is needed. Keep the
  // reclaim as a safety net for a runtime whose user mapping behaved differently.
  if (result.code === 0 && typeof process.getuid === 'function') {
    try {
      if (fsSync.statSync(outDir).uid !== process.getuid()) {
        step('output dir is owned by root — reclaiming it');
        spawnSync('chown', ['-R', `${process.getuid()}:${process.getgid()}`, outDir], { stdio: 'ignore' });
      }
    } catch { /* nothing to reclaim */ }
  }
  return result.code;
}

// ── README publishing ───────────────────────────────────────────────────────
//
// Still PNGs, not video. GitHub strips a <video> tag with a repo-relative src,
// so an mp4 only renders from an absolute URL; a GIF of the full walk would be
// several MB in git. The README already documents itself with viewport-sized
// desktop/mobile PNG pairs, and those are what this produces.

const README_BEGIN = '<!-- ui-capture:begin — regenerate with `node .kilo/skills/ui-capture/scripts/uicapture.mjs --publish` -->';
const README_END = '<!-- ui-capture:end -->';
const README_PREVIEW_BEGIN = '<!-- ui-capture-preview:begin — regenerate with `node .kilo/skills/ui-capture/scripts/uicapture.mjs --publish` -->';
const README_PREVIEW_END = '<!-- ui-capture-preview:end -->';
const SCREENSHOT_DIR = path.join(REPO_ROOT, 'screenshots');

// One entry per screen, and the single source of truth for the README gallery:
// the browser stage shoots exactly this list (passed down as PUBLISH_SCREENS) and
// publishScreens copies exactly this list. `preset` is the key in
// browser-capture.mjs PRESETS, `file` the committed name in screenshots/, and
// `anonymous` marks the screens that must be shot before anything authenticates
// (/web/auth redirects a logged-in user to /web, so login needs its own session).
//
// Keep only pages the demo seed populates; waste and shopping-list rely on
// savings_records and shopping_list_items, which seed.go writes.
const PUBLISHED_SCREENS = [
  { file: 'portal', caption: 'Dashboard — metric tiles, waste rate, category breakdown and expiry trend charts', preset: 'dashboard', anonymous: false, mobile: true },
  { file: 'search', caption: 'Products — search, filter, adjust and manage what is in your household', preset: 'products', anonymous: false, mobile: true },
  { file: 'product-detail', caption: 'Product detail — expiry status, actions and history', preset: 'product-detail', anonymous: false, mobile: true },
  { file: 'create', caption: 'Add product — barcode scan with auto-fill from OpenFoodFacts', preset: 'add-product', anonymous: false, mobile: true },
  { file: 'receipt', caption: 'Receipt scan — OCR bulk entry from a grocery receipt photo', preset: 'receipt-scan', anonymous: false, mobile: true },
  { file: 'recipe', caption: 'Recipes — suggestions built from the products you already have', preset: 'recipes', anonymous: false, mobile: true },
  { file: 'waste', caption: 'Waste analytics — consumed vs wasted, monthly breakdown and cost of waste', preset: 'waste', anonymous: false, mobile: true },
  { file: 'shopping', caption: 'Shopping list — shared household list with low-stock import', preset: 'shopping-list', anonymous: false, mobile: true },
  { file: 'onboarding', caption: 'Onboarding — profile, notifications and household setup', preset: 'onboarding', anonymous: false, mobile: true },
  { file: 'settings', caption: 'Settings — notifications, household, tokens and security', preset: 'settings', anonymous: false, mobile: true },
  { file: 'login', caption: 'Sign in', preset: 'login', anonymous: true, mobile: false },
];

// The ## Preview section near the top of the README. Desktop-only, curated
// subset of the gallery — the above-the-fold pitch. Each entry references a
// file that PUBLISHED_SCREENS already shoots, so no extra capture is needed;
// this list only controls which screens appear and how they are captioned.
const PREVIEW_SCREENS = [
  { file: 'portal', caption: 'Dashboard — metric tiles, waste rate donut, category breakdown and expiry trend charts' },
  { file: 'search', caption: 'Products — search, filter and manage your products' },
];

function renderReadmeBlock(screens) {
  const lines = [README_BEGIN, ''];
  for (const screen of screens) {
    if (screen.mobile) {
      lines.push(
        `<table>`,
        `  <tr>`,
        `    <th>Desktop</th>`,
        `    <th>Mobile</th>`,
        `  </tr>`,
        `  <tr>`,
        `    <td width="50%"><img src="./screenshots/${screen.file}.png" alt="${screen.caption} — desktop"></td>`,
        `    <td width="25%"><img src="./screenshots/${screen.file}_mobile.png" alt="${screen.caption} — mobile"></td>`,
        `  </tr>`,
        `</table>`,
        '',
      );
    } else {
      lines.push(
        `**${screen.caption}**`,
        '',
        `![${screen.caption}](./screenshots/${screen.file}.png)`,
        '',
      );
    }
  }
  lines.push(README_END);
  return lines.join('\n');
}

function renderPreviewBlock(screens) {
  const lines = [README_PREVIEW_BEGIN, ''];
  for (const screen of screens) {
    lines.push(
      `**${screen.caption}**`,
      '',
      `![${screen.caption}](./screenshots/${screen.file}.png)`,
      '',
    );
  }
  lines.push(README_PREVIEW_END);
  return lines.join('\n');
}

// Rewrites only the regions between the markers. Everything outside stays
// hand-curated, so a bad publish can never mangle the rest of the README.
async function publishScreens(outDir) {
  const copied = [];
  const expected = new Set();
  for (const screen of PUBLISHED_SCREENS) {
    for (const suffix of screen.mobile ? ['', '_mobile'] : ['']) {
      const source = path.join(outDir, `${screen.file}${suffix}.png`);
      if (!fsSync.existsSync(source)) {
        throw new UiCaptureError(5, `publish expected ${source} but the capture did not produce it`);
      }
      const destination = path.join(SCREENSHOT_DIR, `${screen.file}${suffix}.png`);
      await fs.copyFile(source, destination);
      copied.push(destination);
      expected.add(path.basename(source));
    }
  }

  // The browser stage shoots PUBLISHED_SCREENS verbatim, so anything left over
  // means the two sides disagreed about the screen set. Surface it instead of
  // silently leaving a stale PNG behind in screenshots/.
  const orphans = (await fs.readdir(outDir))
    .filter((name) => name.endsWith('.png') && !expected.has(name))
    .sort();
  if (orphans.length > 0) {
    throw new UiCaptureError(
      5,
      `the capture produced ${orphans.join(', ')}, which no screen in PUBLISHED_SCREENS claims.\n` +
        '  Either add it to PUBLISHED_SCREENS or drop it from the shoot list.',
    );
  }

  const readmePath = path.join(REPO_ROOT, 'README.md');
  let readme = await fs.readFile(readmePath, 'utf8');

  // Gallery block under ## Screenshots
  const block = renderReadmeBlock(PUBLISHED_SCREENS);
  if (readme.includes(README_BEGIN) || readme.includes(README_END)) {
    if (!readme.includes(README_BEGIN) || !readme.includes(README_END)) {
      throw new UiCaptureError(5, `README.md has only one ui-capture marker; fix the block by hand before publishing`);
    }
    readme = readme.replace(
      new RegExp(`${escapeRegExp(README_BEGIN)}[\\s\\S]*${escapeRegExp(README_END)}`),
      block,
    );
  } else if (readme.includes('## Screenshots')) {
    // First publish: insert immediately under the heading so the generated
    // gallery leads the section and the hand-written entries follow it.
    readme = readme.replace('## Screenshots', `## Screenshots\n\n${block}`);
  } else {
    throw new UiCaptureError(5, 'README.md has no `## Screenshots` heading and no ui-capture markers; add them by hand first');
  }

  // Preview block under ## Preview
  const previewBlock = renderPreviewBlock(PREVIEW_SCREENS);
  if (readme.includes(README_PREVIEW_BEGIN) || readme.includes(README_PREVIEW_END)) {
    if (!readme.includes(README_PREVIEW_BEGIN) || !readme.includes(README_PREVIEW_END)) {
      throw new UiCaptureError(5, `README.md has only one ui-capture-preview marker; fix the block by hand before publishing`);
    }
    readme = readme.replace(
      new RegExp(`${escapeRegExp(README_PREVIEW_BEGIN)}[\\s\\S]*${escapeRegExp(README_PREVIEW_END)}`),
      previewBlock,
    );
  } else if (readme.includes('## Preview')) {
    // First publish: insert immediately under the heading.
    readme = readme.replace('## Preview', `## Preview\n\n${previewBlock}`);
  } else {
    throw new UiCaptureError(5, 'README.md has no `## Preview` heading and no ui-capture-preview markers; add them by hand first');
  }

  await fs.writeFile(readmePath, readme);
  return copied;
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// ── transcodes ──────────────────────────────────────────────────────────────

// The image's bundled ffmpeg is a stripped VP8 build with no H.264 encoder at
// all, so transcoding has to run on the host. Fedora ships ffmpeg-free, which
// has no libx264 either — probe for whatever this host actually offers.
const H264_ENCODERS = ['libx264', 'libopenh264', 'h264_vaapi'];

function h264Encoder() {
  const probe = spawnSync('ffmpeg', ['-hide_banner', '-encoders'], { encoding: 'utf8' });
  if (probe.status !== 0 || !probe.stdout) return '';
  return H264_ENCODERS.find((name) => new RegExp(`^\\s*V[^\\n]*\\s${name}\\s`, 'm').test(probe.stdout)) || '';
}

async function transcode(outDir, opts) {
  const webm = path.join(outDir, `${artifactName(opts)}.webm`);
  const produced = [];
  if (!fsSync.existsSync(webm)) return produced;

  if (!which('ffmpeg')) {
    step('ffmpeg is not on PATH — keeping the webm and all stills, skipping --mp4/--gif');
    return produced;
  }

  if (opts.mp4) {
    const encoder = h264Encoder();
    const mp4 = path.join(outDir, `${artifactName(opts)}.mp4`);
    if (!encoder) {
      step('this ffmpeg build has no H.264 encoder (install ffmpeg-libs for libx264) — keeping the webm');
    } else {
      if (encoder !== 'libx264') step(`libx264 unavailable; transcoding the mp4 with ${encoder}`);
      const result = await run('ffmpeg', [
        '-y', '-loglevel', 'error', '-i', webm,
        '-c:v', encoder, '-pix_fmt', 'yuv420p', '-movflags', '+faststart',
        mp4,
      ], { quiet: true });
      if (result.code === 0) produced.push(mp4);
      else step(`mp4 transcode failed (ffmpeg exit ${result.code}); the webm is intact`);
    }
  }

  if (opts.gif) {
    const palette = path.join(outDir, `.palette-${artifactName(opts)}.png`);
    const gif = path.join(outDir, `${artifactName(opts)}.gif`);
    const gen = await run('ffmpeg', [
      '-y', '-loglevel', 'error', '-i', webm,
      '-vf', 'fps=12,scale=800:-1:flags=lanczos,palettegen=stats_mode=diff',
      palette,
    ], { quiet: true });
    if (gen.code !== 0) {
      step(`gif palettegen failed (ffmpeg exit ${gen.code}); the webm is intact`);
      return produced;
    }
    const use = await run('ffmpeg', [
      '-y', '-loglevel', 'error', '-i', webm, '-i', palette,
      '-lavfi', 'fps=12,scale=800:-1:flags=lanczos[x];[x][1:v]paletteuse=dither=bayer:bayer_scale=4',
      gif,
    ], { quiet: true });
    await fs.rm(palette, { force: true });
    if (use.code === 0) produced.push(gif);
    else step(`gif transcode failed (ffmpeg exit ${use.code}); the webm is intact`);
  }
  return produced;
}

// ── main ────────────────────────────────────────────────────────────────────

async function main() {
  const opts = parseArgs(process.argv.slice(2));
  if (opts.help) {
    out(USAGE);
    return 0;
  }

  const info = preflight(opts);
  step(`playwright ${info.version} → ${info.image || '(local browser)'}`);

  if (opts.script) {
    opts.script = path.resolve(REPO_ROOT, opts.script);
    if (!fsSync.existsSync(opts.script)) throw new UiCaptureError(1, `--script file not found: ${opts.script}`);
    // Container mode mounts only the repo, so a flow outside it cannot be reached.
    if (opts.browser === 'container' && path.relative(REPO_ROOT, opts.script).startsWith('..')) {
      throw new UiCaptureError(1, `--script must live inside the repo for --browser container: ${opts.script}`);
    }
  }

  const port = opts.port === 0 ? await freePort() : opts.port;
  // browserEnv() reads opts.port, so replace 0 with the port actually chosen —
  // proviant.go:238 maps a non-positive server.port back to 5114.
  opts.port = port;
  if (!(await portIsFree(port))) {
    throw new UiCaptureError(
      2,
      `Port ${port} is already in use.\n` +
        '  Stop the process holding it (task run uses 5114), pass --port <n>, or pass\n' +
        '  --port 0 to let the OS pick an ephemeral port.',
    );
  }

  const outDir = path.resolve(
    REPO_ROOT,
    opts.out || path.join('.cache', 'ui-captures', `${opts.preset}-${stamp()}`),
  );
  await fs.mkdir(outDir, { recursive: true });

  const tmp = await fs.mkdtemp(path.join(os.tmpdir(), 'uicapture-'));
  const binDir = path.join(tmp, 'bin');
  const serverDir = path.join(tmp, 'server');
  const baseURL = `http://127.0.0.1:${port}`;
  let server = null;
  let exitCode = 0;

  try {
    await fs.mkdir(binDir, { recursive: true });
    await fs.mkdir(path.join(serverDir, 'data'), { recursive: true });

    step('building proviant and the demo seeder');
    const build = await run('go', ['build', '-o', path.join(binDir, 'proviant'), '.'], {
      cwd: path.join(REPO_ROOT, 'src'),
    });
    if (build.code !== 0) throw new UiCaptureError(5, `go build failed (exit ${build.code})`);
    // seed.go sits outside src/, which is the module root, hence the ../ path.
    const seedBuild = await run('go', ['build', '-o', path.join(binDir, 'proviant-seed'), '../seed.go'], {
      cwd: path.join(REPO_ROOT, 'src'),
    });
      if (seedBuild.code !== 0) throw new UiCaptureError(5, `go build of the seeder failed (exit ${seedBuild.code})`);

    step('seeding demo data');
    const seed = await run(path.join(binDir, 'proviant-seed'), [], { cwd: path.join(serverDir, 'data') });
    if (seed.code !== 0) throw new UiCaptureError(5, `demo seed failed (exit ${seed.code})`);

    // Copied verbatim on purpose: viper's AutomaticEnv (proviant.go:107-109)
    // overrides any key the file already declares, so a hand-rewritten copy
    // could only drift. Env overrides below carry every intentional change.
    step(`config from ${path.relative(REPO_ROOT, info.configSource)}`);
    await fs.copyFile(info.configSource, path.join(serverDir, 'config.yaml'));

    step(`starting server on ${baseURL}`);
    server = spawnServer(path.join(binDir, 'proviant'), serverDir, serverEnv(port));
    await waitForServer(server.child, baseURL, server.logFile, opts.timeout);

    const browserCode = await runBrowser(info, opts, outDir);
    if (browserCode !== 0) {
      exitCode = browserCode;
    } else if (opts.publish) {
      const copied = await publishScreens(outDir);
      step(`published ${copied.length} screenshot(s) into screenshots/`);
      for (const file of copied) out(file);
      out(path.join(REPO_ROOT, 'README.md'));
    } else {
      await transcode(outDir, opts);
    }
  } finally {
    await stopServer(server);
    if (opts.keepServer) step(`kept temp server dir: ${serverDir}`);
    else await fs.rm(tmp, { recursive: true, force: true });
  }

  const files = (await fs.readdir(outDir))
    .filter((name) => !name.startsWith('.'))
    .sort()
    .map((name) => path.join(outDir, name));
  if (files.length > 0) {
    out('');
    for (const file of files) out(file);
  }
  const stills = files.filter((file) => file.endsWith('.png')).length;
  out('');
  out(`${stills} still(s)${files.some((f) => f.endsWith('.webm')) ? ' + webm' : ''} in ${outDir}`);
  return exitCode;
}

main()
  .then((code) => process.exit(code))
  .catch((error) => {
    const known = error instanceof UiCaptureError;
    err(`uicapture: ${error.message}`);
    if (!known) err(error.stack || '');
    process.exit(known ? error.code : 1);
  });