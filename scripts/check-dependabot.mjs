#!/usr/bin/env node
// ─────────────────────────────────────────────────────────────────────────────
// scripts/check-dependabot.mjs
//
// Acceptance gate for the Dependabot configuration and the invariants a
// proposal must not break.
//
//   node scripts/check-dependabot.mjs
//
//   1. .github/dependabot.yml parses and covers gomod, npm, docker and
//      github-actions on a weekly Monday schedule.
//   2. Every action in .github/workflows and in .github/actions is pinned to a
//      commit SHA — a proposal that replaces a SHA with a tag fails here (Sonar
//      finding githubactions:S7637).
//   3. The CI runs this gate in the ops-check job, and its image-scan job
//      skips Dependabot pull requests.
//   4. The trivy image is pinned by digest and every place that carries the
//      pin agrees: scripts/image-scan.sh and the Taskfile.
//   5. The npm audit policy is untouched: it lives once, in
//      scripts/npm-audit.sh, with the production-only gate at
//      --audit-level=high, the informational dev audit, no
//      `npm audit fix --force` and no `braces` overrides entry.
//   6. Bootstrap stays excluded from the automated npm updates (alpha policy).
//   7. The Go toolchain version is the same in both Dockerfiles and in both
//      workflows — a base-image bump that skips the workflows builds the
//      image with a different compiler than CI tests with.
//   8. Both Dockerfiles run on the same alpine base image.
//   9. The Node major is the same in the manifest, in .nvmrc, in the workflows,
//      in the Dockerfiles and in the interpreter that runs this script — an
//      image built with a different Node than CI tests with is the same silent
//      divergence the Go check above exists to catch.
//
// The Monday schedule is fixed policy, not a preference: the README documents
// it, so moving a day means moving the day in the config, here and there.
//
// Exit code 0 = all checks pass.
// ─────────────────────────────────────────────────────────────────────────────

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import yaml from 'js-yaml';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

// ── Reporting ───────────────────────────────────────────────────────────────

const failures = [];

const fail = (check, message) => {
  const entry = `${check}: ${message}`;
  if (!failures.includes(entry)) failures.push(entry);
};

function read(relative) {
  return fs.readFileSync(path.join(ROOT, relative), 'utf8');
}

function readOptional(relative) {
  try {
    return read(relative);
  } catch {
    return null;
  }
}

function readJson(relative, check) {
  const raw = readOptional(relative);
  if (raw === null) {
    fail(check, `${relative} is missing`);
    return null;
  }
  try {
    return JSON.parse(raw);
  } catch (error) {
    fail(check, `${relative} does not parse: ${error.message}`);
    return null;
  }
}

const isComment = (line) => {
  const trimmed = line.trimStart();
  return trimmed.startsWith('#') || trimmed.startsWith('//');
};

// A block comment is prose too, whatever its opening looks like.
const stripBlockComments = (content) => content.replace(/\/\*[\s\S]*?\*\//g, '');

// Assembled from parts so this file never holds the literal in a line the
// check would read as a call.
const FORBIDDEN_AUDIT_FIX = ['npm audit', 'fix --force'].join(' ');
const AUDIT_COMMAND = ['npm', 'audit'].join(' ');

// A line that runs the audit, not a line that names it: the script's header
// quotes the command in prose and the warning it prints names it a third time.
// A quoted occurrence is a command only inside a shell wrapper — `sh -c "npm
// audit …"` runs it, `echo "… npm audit …"` prints it.
const SHELL_WRAPPER = /\b(?:sh|bash|zsh|dash)\s+-[a-z]*c\b|\beval\b/;

const runsAudit = (line) => {
  if (!line.includes(AUDIT_COMMAND)) return false;
  const withoutStrings = line.replace(/'[^']*'/g, '').replace(/"[^"]*"/g, '').replace(/`[^`]*`/g, '');
  if (withoutStrings.includes(AUDIT_COMMAND)) return true;
  return SHELL_WRAPPER.test(line);
};

function collectFiles(directory) {
  const collected = [];
  if (!fs.existsSync(directory)) return collected;
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const absolute = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      collected.push(...collectFiles(absolute));
      continue;
    }
    collected.push(path.relative(ROOT, absolute));
  }
  return collected;
}

// Every string under a given key, wherever it sits in the document.
function collectStringValues(node, key, found) {
  if (!node || typeof node !== 'object') return;
  for (const [name, value] of Object.entries(node)) {
    if (name === key) {
      if (typeof value === 'string') {
        found.push(value);
      } else if (Array.isArray(value)) {
        for (const item of value) {
          if (typeof item === 'string') found.push(item);
        }
      }
      continue;
    }
    collectStringValues(value, key, found);
  }
}

function workflowFiles() {
  return collectFiles(path.join(ROOT, '.github', 'workflows')).filter((file) => file.endsWith('.yml') || file.endsWith('.yaml'));
}

function actionFiles() {
  return collectFiles(path.join(ROOT, '.github', 'actions')).filter((file) => file.endsWith('.yml') || file.endsWith('.yaml'));
}

function parseYaml(file, check) {
  const raw = readOptional(file);
  if (raw === null) {
    fail(check, `${file} is missing`);
    return null;
  }
  let parsed;
  try {
    parsed = yaml.load(raw);
  } catch (error) {
    fail(check, `${file} does not parse: ${error.message}`);
    return null;
  }
  if (!parsed || typeof parsed !== 'object') {
    fail(check, `${file} is empty`);
    return null;
  }
  return parsed;
}

// ── 1. The configuration ────────────────────────────────────────────────────

const ECOSYSTEMS = {
  gomod: '/src',
  npm: '/',
  docker: '/',
  'github-actions': '/',
};

// The keys an update entry may carry. GitHub rejects the whole file when one
// of them is unknown, so a typo here would disable every ecosystem silently.
const UPDATE_KEYS = new Set([
  'allow',
  'assignees',
  'commit-message',
  'cooldown',
  'directories',
  'directory',
  'exclude-paths',
  'groups',
  'ignore',
  'insecure-external-code-execution',
  'labels',
  'milestone',
  'name',
  'open-pull-requests-limit',
  'package-ecosystem',
  'pull-request-branch-name',
  'rebase-strategy',
  'registries',
  'schedule',
  'target-branch',
  'vendor',
  'versioning-strategy',
]);

const TOP_LEVEL_KEYS = new Set(['version', 'enable-beta-ecosystems', 'updates', 'registries', 'multi-ecosystem-groups']);

function parseConfig() {
  const raw = readOptional('.github/dependabot.yml');
  if (raw === null) {
    fail('config', '.github/dependabot.yml is missing');
    return null;
  }

  let parsed;
  try {
    parsed = yaml.load(raw);
  } catch (error) {
    fail('config', `.github/dependabot.yml does not parse: ${error.message}`);
    return null;
  }
  // An empty or comment-only file parses to null: without this the gate would
  // pass a configuration that configures nothing.
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    fail('config', '.github/dependabot.yml is empty or not a mapping');
    return null;
  }
  return parsed;
}

function checkConfiguration(config) {
  if (!config) return;
  if (config.version !== 2) fail('config', `version must be 2, got ${JSON.stringify(config.version)}`);

  for (const key of Object.keys(config)) {
    if (!TOP_LEVEL_KEYS.has(key)) fail('config', `unknown top-level key "${key}"`);
  }

  if (!Array.isArray(config.updates)) {
    fail('config', 'updates must be a list of update entries');
    return;
  }
  const updates = config.updates;
  if (updates.length === 0) {
    fail('config', 'no update entries');
    return;
  }

  for (const [ecosystem, directory] of Object.entries(ECOSYSTEMS)) {
    const entry = updates.find((update) => update?.['package-ecosystem'] === ecosystem);
    if (!entry) {
      fail('config', `no update entry for ${ecosystem}`);
      continue;
    }
    for (const key of Object.keys(entry)) {
      if (!UPDATE_KEYS.has(key)) fail('config', `${ecosystem}: unknown key "${key}"`);
    }
    if (entry.schedule?.interval !== 'weekly') {
      fail('config', `${ecosystem}: schedule.interval must be "weekly"`);
    }
    // The README documents a Monday schedule; the day belongs with it.
    if (entry.schedule?.day !== 'monday') {
      fail('config', `${ecosystem}: schedule.day must be "monday"`);
    }
    if (entry.directory !== directory) {
      fail('config', `${ecosystem}: directory must be "${directory}", got ${JSON.stringify(entry.directory)}`);
    }
    // The proposals are batched: without groups Dependabot opens one pull
    // request per dependency.
    const groups = entry.groups;
    if (!groups || typeof groups !== 'object' || Object.keys(groups).length === 0) {
      fail('config', `${ecosystem}: the updates must be grouped into pull requests`);
    } else {
      for (const name of Object.keys(groups)) {
        if (!/^[A-Za-z](?:[A-Za-z|_-]*[A-Za-z])?$/.test(name)) {
          fail('config', `${ecosystem}: "${name}" is not a valid group name`);
        }
      }
    }
    if (entry['target-branch'] !== undefined && entry['target-branch'] !== 'main') {
      fail('config', `${ecosystem}: target-branch must be "main"`);
    }
    if (entry.directory !== undefined && entry.directories !== undefined) {
      fail('config', `${ecosystem}: directory and directories cannot both be set`);
    }
  }

  const seen = new Set();
  for (const update of updates) {
    const ecosystem = update?.['package-ecosystem'];
    if (!ecosystem) {
      fail('config', 'an update entry has no package-ecosystem');
      continue;
    }
    if (!Object.hasOwn(ECOSYSTEMS, ecosystem)) {
      fail('config', `unexpected ecosystem: ${ecosystem} — extend ECOSYSTEMS in the checker if it should be monitored`);
      continue;
    }
    if (seen.has(ecosystem)) {
      fail('config', `${ecosystem}: more than one update entry — only the first one is checked here`);
      continue;
    }
    seen.add(ecosystem);
  }

  const npm = updates.find((update) => update?.['package-ecosystem'] === 'npm');
  if (npm) {
    if (npm['versioning-strategy'] !== 'increase-if-necessary') {
      fail('config', 'npm: versioning-strategy must be "increase-if-necessary"');
    }
    // The ignore has to cover the whole dependency: an ignore that names
    // versions or update types leaves the other releases open for a proposal.
    const rules = Array.isArray(npm.ignore) ? npm.ignore : [];
    const rule = rules.find((candidate) => candidate?.['dependency-name'] === 'bootstrap');
    if (!rule) {
      fail('bootstrap', 'the npm entry must ignore "bootstrap" — Bootstrap 6 is an alpha');
    } else if (Object.keys(rule).length !== 1) {
      fail('bootstrap', 'the bootstrap ignore must name the dependency and nothing else');
    }
  }
}

// ── 2. The action pins ──────────────────────────────────────────────────────

// The references are read from the parsed document, not from the text: a step
// that echoes "uses: owner/action@v1" in its run block is not an action, and a
// step input named "uses" is not an action either.
function collectActionReferences(document) {
  const references = [];
  const collectSteps = (steps) => {
    if (!Array.isArray(steps)) return;
    for (const step of steps) {
      if (step && typeof step.uses === 'string') references.push(step.uses);
    }
  };

  const jobs = document?.jobs;
  if (jobs && typeof jobs === 'object') {
    for (const job of Object.values(jobs)) {
      if (!job || typeof job !== 'object') continue;
      if (typeof job.uses === 'string') references.push(job.uses); // a reusable workflow
      collectSteps(job.steps);
    }
  }
  collectSteps(document?.runs?.steps); // a composite action
  return references;
}

function collectStepRuns(document) {
  const runs = [];
  const collectSteps = (steps) => {
    if (!Array.isArray(steps)) return;
    for (const step of steps) {
      if (step && typeof step.run === 'string') runs.push(step.run);
    }
  };

  const jobs = document?.jobs;
  if (jobs && typeof jobs === 'object') {
    for (const job of Object.values(jobs)) {
      if (job && typeof job === 'object') collectSteps(job.steps);
    }
  }
  collectSteps(document?.runs?.steps);
  return runs;
}

function checkActionPins() {
  const files = [...workflowFiles(), ...actionFiles()];
  if (files.length === 0) {
    fail('pins', 'no workflow or action files found under .github');
    return;
  }

  for (const file of files) {
    const document = parseYaml(file, 'pins');
    if (!document) continue;

    for (const reference of collectActionReferences(document)) {
      if (reference.startsWith('./')) continue; // a local composite action
      if (reference.startsWith('docker://')) continue; // a container image, not an action
      const separator = reference.lastIndexOf('@');
      if (separator < 0) {
        fail('pins', `${file}: ${reference} has no version reference`);
        continue;
      }
      const action = reference.slice(0, separator);
      const ref = reference.slice(separator + 1);
      if (!/^[0-9a-f]{40}$/.test(ref)) {
        fail('pins', `${file}: ${action} is pinned to "${ref}" instead of a commit SHA`);
      }
    }
  }
}

// ── 3. The CI wiring ────────────────────────────────────────────────────────

function checkCiWiring() {
  const document = parseYaml('.github/workflows/ci.yml', 'ci');
  if (!document) return;

  const gate = document.jobs?.['ops-check'];
  if (!gate) {
    fail('ci', 'the CI has no ops-check job — the dependency-pinning gate would never run');
  } else {
    if (gate.if !== undefined) {
      fail('ci', 'the ops-check job must not carry a condition — it would let the gate be skipped');
    }
    const steps = Array.isArray(gate.steps) ? gate.steps : [];
    const runs = steps.some((step) => typeof step.run === 'string' && step.run.includes('scripts/check-dependabot.mjs'));
    if (!runs) fail('ci', 'the ops-check job does not run scripts/check-dependabot.mjs');
  }

  const condition = document.jobs?.['image-scan']?.if;
  if (typeof condition !== 'string') {
    fail('ci', 'the image-scan job has no condition');
    return;
  }
  // The comparison has to name the pull request author: the branch of a
  // Dependabot proposal is named after the dependency, not after the bot.
  if (!/github\.event\.pull_request\.user\.login\s*!=\s*['"]?dependabot\[bot\]['"]?/.test(condition)) {
    fail('ci', 'the image-scan job must skip Dependabot pull requests — their workflow gets a read-only token');
  }
}

// ── 4. The trivy digest ─────────────────────────────────────────────────────

function checkTrivyDigest() {
  const script = readOptional('scripts/image-scan.sh');
  const taskfile = readOptional('Taskfile.yml');
  if (script === null || taskfile === null) {
    fail('digest', 'scripts/image-scan.sh or Taskfile.yml is missing');
    return;
  }

  // The values are read line by line rather than with a pattern, so a
  // reformatting of either file cannot make the pin look missing. Every
  // assignment counts: the last one wins at runtime.
  const stripQuotes = (value) => value.replace(/^["']/, '').replace(/["']$/, '');
  const valuesAfter = (content, marker) =>
    content
      .split('\n')
      .filter((line) => !isComment(line) && line.includes(marker))
      .map((line) => stripQuotes(line.slice(line.indexOf(marker) + marker.length).trim()));

  const inScript = valuesAfter(script, 'TRIVY_IMAGE=').map(
    (value) => value.match(/\$\{TRIVY_IMAGE:-([^}]+)\}/)?.[1] ?? value,
  );
  const inTaskfile = valuesAfter(taskfile, 'TRIVY_IMAGE:');

  if (inScript.length === 0 || inTaskfile.length === 0) {
    fail('digest', 'the trivy digest pin is missing from scripts/image-scan.sh or Taskfile.yml');
    return;
  }
  if (new Set([...inScript, ...inTaskfile]).size > 1) {
    fail(
      'digest',
      `the trivy pin is not the same everywhere: scripts/image-scan.sh ${inScript.join(', ')} vs Taskfile.yml ${inTaskfile.join(', ')}`,
    );
    return;
  }
  // Equal is not enough: a mutable tag in both places is the regression the
  // digest pin exists to prevent.
  if (!/@sha256:[0-9a-f]{64}$/.test(inScript[0])) {
    fail('digest', `the trivy image must be pinned by digest, got "${inScript[0]}"`);
  }
}

// ── 5. The npm audit policy ─────────────────────────────────────────────────

function mentionsBraces(node) {
  if (!node || typeof node !== 'object') return false;
  return Object.entries(node).some(
    ([key, value]) => key.includes('braces') || (typeof value === 'string' && value.includes('braces')) || mentionsBraces(value),
  );
}

function checkNpmAuditPolicy() {
  const audit = readOptional('scripts/npm-audit.sh');
  if (audit === null) {
    fail('npm-audit', 'scripts/npm-audit.sh is missing');
    return;
  }

  // A line continuation can split the command in two, so the lines are joined
  // before the search.
  const collapsed = stripBlockComments(audit).replace(/\\\r?\n/g, ' ');
  const invocations = collapsed.split('\n').filter((line) => !isComment(line) && runsAudit(line));
  const gate = invocations.find((line) => line.includes('--omit=dev'));
  if (!gate) {
    fail('npm-audit', 'scripts/npm-audit.sh lost the production-only gate (--omit=dev)');
  } else if (!gate.includes('--audit-level=high')) {
    fail('npm-audit', 'the production gate must audit at --audit-level=high');
  }
  if (!invocations.some((line) => !line.includes('--omit=dev'))) {
    fail('npm-audit', 'scripts/npm-audit.sh lost the informational dev-dependency audit');
  }

  const manifest = readJson('package.json', 'npm-audit');
  if (manifest) {
    const overrides = { ...manifest.overrides, ...manifest.resolutions };
    if (mentionsBraces(overrides)) {
      fail('npm-audit', 'package.json must not carry a braces override or resolution');
    }
  }

  // The policy lives once, in scripts/npm-audit.sh. An audit inlined into a
  // workflow, a task, a script or a package script drifts from it without
  // failing anything.
  for (const file of [...workflowFiles(), ...actionFiles()]) {
    const document = parseYaml(file, 'npm-audit');
    if (!document) continue;
    for (const run of collectStepRuns(document)) {
      if (runsAudit(run)) fail('npm-audit', `${file} inlines an npm audit — the policy lives in scripts/npm-audit.sh`);
    }
  }

  const taskfile = parseYaml('Taskfile.yml', 'npm-audit');
  if (taskfile) {
    const commands = [];
    collectStringValues(taskfile, 'cmds', commands);
    collectStringValues(taskfile, 'cmd', commands);
    for (const command of commands) {
      if (runsAudit(command)) fail('npm-audit', 'Taskfile.yml inlines an npm audit — the policy lives in scripts/npm-audit.sh');
    }
  }

  for (const file of collectFiles(path.join(ROOT, 'scripts'))) {
    if (file === 'scripts/npm-audit.sh') continue;
    const content = readOptional(file);
    if (content === null) continue;
    const collapsed = stripBlockComments(content).replace(/\\\r?\n/g, ' ');
    if (collapsed.split('\n').some((line) => !isComment(line) && runsAudit(line))) {
      fail('npm-audit', `${file} inlines an npm audit — the policy lives in scripts/npm-audit.sh`);
    }
  }

  if (manifest) {
    for (const command of Object.values(manifest.scripts ?? {})) {
      if (typeof command === 'string' && runsAudit(command)) {
        fail('npm-audit', 'package.json inlines an npm audit — the policy lives in scripts/npm-audit.sh');
      }
    }
  }

  const sources = [
    ...collectFiles(path.join(ROOT, 'scripts')),
    ...collectFiles(path.join(ROOT, '.github')),
    'Taskfile.yml',
    'package.json',
  ];
  for (const file of sources) {
    const content = readOptional(file);
    if (content === null) continue;
    // A line continuation can split the command in two, so the lines are
    // joined before the search. The policy text names the forbidden command in
    // a comment; only a line that would run it counts.
    const collapsed = stripBlockComments(content).replace(/\\\r?\n/g, ' ');
    const executes = collapsed.split('\n').some((line) => !isComment(line) && line.includes(FORBIDDEN_AUDIT_FIX));
    if (executes) fail('npm-audit', `${file} runs "${FORBIDDEN_AUDIT_FIX}"`);
  }
}

// ── 6. The alpha exclusion ──────────────────────────────────────────────────

function checkBootstrapExcluded(config) {
  if (!config) return; // the parse failure is already reported
  const updates = Array.isArray(config.updates) ? config.updates : [];
  const npm = updates.find((update) => update?.['package-ecosystem'] === 'npm');
  const rules = Array.isArray(npm?.ignore) ? npm.ignore : [];
  const ignored = rules.some((rule) => rule?.['dependency-name'] === 'bootstrap');
  if (!ignored) fail('bootstrap', 'the npm entry must ignore "bootstrap" — Bootstrap 6 is an alpha');

  const manifest = readJson('package.json', 'bootstrap');
  if (manifest && !manifest.dependencies?.bootstrap) {
    fail('bootstrap', 'package.json no longer lists bootstrap as a dependency — the ignore rule matches nothing');
  }
}

// ── 7. The Go toolchain and the base image ──────────────────────────────────

function checkGoToolchain() {
  // The source of a version is a FROM line: a comment that names a version
  // does not pull an image.
  const fromLines = (content) => content.split('\n').filter((line) => /^\s*FROM\s/i.test(line));

  const sources = new Map();
  const record = (source, version) => {
    if (!sources.has(source)) sources.set(source, new Set());
    sources.get(source).add(version);
  };

  for (const file of ['Dockerfile', 'Dockerfile.demo']) {
    const content = readOptional(file);
    if (content === null) {
      fail('go-toolchain', `${file} is missing`);
      continue;
    }
    const found = fromLines(content).flatMap((line) =>
      [...line.matchAll(/golang:([0-9]+(?:\.[0-9]+)*)/g)].map((match) => match[1]),
    );
    if (found.length === 0) {
      fail('go-toolchain', `${file}: the golang base image has no literal version — the toolchain check needs one`);
      continue;
    }
    for (const version of found) record(file, version);
  }

  for (const file of [
    '.github/workflows/ci.yml',
    '.github/workflows/weekly-vuln-scan.yml',
    '.github/workflows/weekly-backup-drill.yml',
  ]) {
    const content = readOptional(file);
    if (content === null) {
      fail('go-toolchain', `${file} is missing`);
      continue;
    }
    // Every occurrence counts: a job-level env: block can override the
    // top-level one. A comment does not count.
    const found = content
      .split('\n')
      .filter((line) => !isComment(line))
      .flatMap((line) => [...line.matchAll(/GO_VERSION:\s*"?([0-9]+(?:\.[0-9]+)*)"?/g)].map((match) => match[1]));
    if (found.length === 0) {
      fail('go-toolchain', `${file}: no GO_VERSION found`);
      continue;
    }
    for (const version of found) record(file, version);
  }

  const versions = new Set([...sources.values()].flatMap((set) => [...set]));
  if (versions.size > 1) {
    const detail = [...sources].map(([source, set]) => `${source}: ${[...set].join('/')}`).join(', ');
    fail('go-toolchain', `the Go toolchain version is not the same everywhere — ${detail}`);
  }

  // The runtime base image is the other half of the image: the production and
  // the demo image must not drift onto different alpine versions.
  const baseImages = new Map();
  for (const file of ['Dockerfile', 'Dockerfile.demo']) {
    const content = readOptional(file);
    if (content === null) continue;
    const versions = fromLines(content).flatMap((line) =>
      [...line.matchAll(/alpine:([0-9]+(?:\.[0-9]+)*)/g)].map((match) => match[1]),
    );
    baseImages.set(file, new Set(versions));
  }
  const alpine = new Set([...baseImages.values()].flatMap((set) => [...set]));
  if (baseImages.size !== 2 || alpine.size !== 1 || [...baseImages.values()].some((set) => set.size !== 1)) {
    const detail = [...baseImages].map(([file, set]) => `${file}: ${[...set].join('/') || 'none'}`).join(', ');
    fail('base-image', `both Dockerfiles must run on the same alpine base image — ${detail}`);
  }
}

// ── 9. The Node toolchain ───────────────────────────────────────────────────

// The manifest names the major; every other surface has to name the same one.
// The range is ">=X.Y.Z <Y+1": the lower bound is the major the repository runs
// on, and the upper bound keeps the range off the next one.
const NODE_RANGE = /^>=(\d+)\.(\d+)\.(\d+) <(\d+)$/;

// Assembled from parts: inside a template literal the expression would be read
// as a JavaScript interpolation.
const NODE_VERSION_EXPRESSION = '${{ env.NODE_VERSION }}';

function checkNodeToolchain() {
  const manifest = readJson('package.json', 'node-toolchain');
  const engines = manifest?.engines?.node;
  if (typeof engines !== 'string') {
    fail('node-toolchain', 'package.json declares no engines.node — the Node version is not pinned');
    return;
  }
  const range = engines.match(NODE_RANGE);
  if (!range) {
    fail('node-toolchain', `engines.node must read ">=X.Y.Z <Y+1", got "${engines}"`);
    return;
  }
  const major = range[1];
  if (range[4] !== String(Number(major) + 1)) {
    fail('node-toolchain', `engines.node must end at the next major (<${Number(major) + 1}), got "${engines}"`);
    return;
  }

  const record = (source, version) => {
    if (String(version) !== major) {
      fail('node-toolchain', `${source} pins Node ${version}, the manifest says ${major}`);
    }
  };

  const nvmrc = readOptional('.nvmrc');
  if (nvmrc === null) {
    fail('node-toolchain', '.nvmrc is missing — the local Node version is not pinned');
  } else if (nvmrc.trim() === '') {
    fail('node-toolchain', '.nvmrc is empty — the local Node version is not pinned');
  } else {
    record('.nvmrc', nvmrc.trim());
  }

  // The build stage pins the major with an apk constraint: Alpine's nodejs
  // package is the only Node in the image, and the constraint fails the build
  // when the branch moves to the next major.
  for (const file of ['Dockerfile', 'Dockerfile.demo']) {
    const content = readOptional(file);
    if (content === null) {
      fail('node-toolchain', `${file} is missing`);
      continue;
    }
    const found = [...content.matchAll(/nodejs=~(\d+)/g)].map((match) => match[1]);
    if (found.length === 0) {
      fail('node-toolchain', `${file} does not pin the Node major (nodejs=~NN)`);
      continue;
    }
    for (const version of found) record(file, version);
  }

  // A workflow that sets up Node reads the major from the workflow env, so
  // there is one place to change and no literal to drift.
  for (const file of workflowFiles()) {
    const document = parseYaml(file, 'node-toolchain');
    if (!document) continue;
    const steps = Object.values(document.jobs ?? {}).flatMap((job) => job?.steps ?? []);
    const setups = steps.filter((step) => step?.uses?.startsWith('actions/setup-node'));
    if (setups.length === 0) continue;
    const version = document.env?.NODE_VERSION;
    if (version === undefined) {
      fail('node-toolchain', `${file} sets up Node but defines no NODE_VERSION`);
      continue;
    }
    record(`${file} NODE_VERSION`, version);
    for (const step of setups) {
      if (step.with?.['node-version'] !== NODE_VERSION_EXPRESSION) {
        fail(
          'node-toolchain',
          `${file}: setup-node asks for "${step.with?.['node-version']}" instead of ${NODE_VERSION_EXPRESSION}`,
        );
      }
    }
  }

  // The interpreter that runs this script — the last surface to drift, and the
  // one a developer feels first.
  record('the running Node', process.versions.node.split('.')[0]);
}

// ── Run ─────────────────────────────────────────────────────────────────────

const config = parseConfig();

checkConfiguration(config);
checkActionPins();
checkCiWiring();
checkTrivyDigest();
checkNpmAuditPolicy();
checkBootstrapExcluded(config);
checkGoToolchain();
checkNodeToolchain();

if (failures.length > 0) {
  console.error(`\ncheck-dependabot: ${failures.length} problem(s)`);
  for (const message of failures) console.error(`  ✗ ${message}`);
  process.exit(1);
}

console.log('check-dependabot: all checks passed');
