#!/usr/bin/env node
// ─────────────────────────────────────────────────────────────────────────────
// scripts/check-workflows.mjs
//
// Acceptance gate for the GitHub Actions workflow files.
//
//   node scripts/check-workflows.mjs
//
//   1. Every workflow in .github/workflows and every action in
//      .github/actions parses as YAML.
//   2. Every workflow names at least one job and every job names at least one
//      step.
//
// A workflow that does not parse is rejected by GitHub Actions, and the
// rejection is silent until the scheduled run never happens: nothing in the
// repository parses these files, so a typo in a `run:` line lands unnoticed.
// A plain scalar that starts with a quote is the shape that breaks it —
// `run: "$BIN" backup-drill` is a quoted scalar followed by trailing text.
//
// Exit code 0 = all checks pass.
// ─────────────────────────────────────────────────────────────────────────────

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import yaml from 'js-yaml';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

function collectFiles(directory) {
  if (!fs.existsSync(directory)) {
    return [];
  }
  return fs
    .readdirSync(directory, { withFileTypes: true })
    .flatMap((entry) => {
      const full = path.join(directory, entry.name);
      if (entry.isDirectory()) {
        return collectFiles(full);
      }
      return entry.name.endsWith('.yml') || entry.name.endsWith('.yaml') ? [full] : [];
    });
}

const workflowFiles = collectFiles(path.join(ROOT, '.github', 'workflows'));
const actionFiles = collectFiles(path.join(ROOT, '.github', 'actions'));
const files = [...workflowFiles, ...actionFiles];

if (workflowFiles.length === 0) {
  console.error('check-workflows: no workflow files found');
  process.exit(1);
}

const failures = [];

for (const file of files) {
  const relative = path.relative(ROOT, file);
  let document;
  try {
    document = yaml.load(fs.readFileSync(file, 'utf8'));
  } catch (error) {
    failures.push(`${relative}: does not parse — ${error.message.split('\n')[0]}`);
    continue;
  }

  if (!document || typeof document !== 'object') {
    failures.push(`${relative}: holds no workflow`);
    continue;
  }

  // An action is not a workflow: it carries the steps of one composite action
  // and no jobs.
  if (actionFiles.includes(file)) {
    if (!document.runs) {
      failures.push(`${relative}: the action names no run`);
    }
    continue;
  }

  // GitHub reads the trigger under the key `on`, which YAML 1.1 parses as
  // the boolean true. Both spellings are the same key.
  const triggers = document.on ?? document[true];
  if (!triggers) {
    failures.push(`${relative}: names no trigger`);
  }

  const jobs = document.jobs;
  if (!jobs || typeof jobs !== 'object' || Object.keys(jobs).length === 0) {
    failures.push(`${relative}: names no job`);
    continue;
  }

  for (const [name, job] of Object.entries(jobs)) {
    if (!job || typeof job !== 'object') {
      failures.push(`${relative}: the job ${name} holds no configuration`);
      continue;
    }
    const steps = job.steps;
    if (!Array.isArray(steps) || steps.length === 0) {
      failures.push(`${relative}: the job ${name} names no step`);
    }
  }
}

if (failures.length > 0) {
  console.error('check-workflows: the workflow files are broken');
  for (const failure of failures) {
    console.error(`  - ${failure}`);
  }
  process.exit(1);
}

console.log(`check-workflows: ${files.length} workflow files parse`);
