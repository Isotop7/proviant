import { describe, expect, it } from "vitest";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const REPO = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");

const HAS_NODE_MODULES = fs.existsSync(path.join(REPO, "node_modules", "js-yaml"));

if (!HAS_NODE_MODULES) {
  console.warn("checkDependabot: skipped — the npm dependencies are missing, run 'task init' first");
}

const ROOT_FILES = ["package.json", "Taskfile.yml", "Dockerfile", "Dockerfile.demo"];

function repoFile(file) {
  return fs.readFileSync(path.join(REPO, file), "utf8");
}

function copyTree(source, target) {
  fs.mkdirSync(target, { recursive: true });
  for (const entry of fs.readdirSync(source, { withFileTypes: true })) {
    const from = path.join(source, entry.name);
    const to = path.join(target, entry.name);
    if (entry.isDirectory()) {
      copyTree(from, to);
      continue;
    }
    fs.copyFileSync(from, to);
  }
}

function buildTree(overrides) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "proviant-dependabot-"));
  try {
    fs.symlinkSync(path.join(REPO, "node_modules"), path.join(root, "node_modules"), "dir");

    copyTree(path.join(REPO, ".github"), path.join(root, ".github"));
    copyTree(path.join(REPO, "scripts"), path.join(root, "scripts"));
    for (const file of ROOT_FILES) {
      const target = path.join(root, file);
      fs.mkdirSync(path.dirname(target), { recursive: true });
      fs.copyFileSync(path.join(REPO, file), target);
    }

    for (const [file, content] of Object.entries(overrides)) {
      const target = path.join(root, file);
      if (content === null) {
        fs.rmSync(target, { force: true });
        continue;
      }
      fs.mkdirSync(path.dirname(target), { recursive: true });
      fs.writeFileSync(target, content);
    }
  } catch (error) {
    fs.rmSync(root, { recursive: true, force: true });
    throw error;
  }
  return root;
}

function runCheck(overrides = {}) {
  const root = buildTree(overrides);
  try {
    const result = spawnSync("node", [path.join(root, "scripts", "check-dependabot.mjs")], {
      encoding: "utf8",
      timeout: 30000,
    });
    if (result.error) throw result.error;
    return { status: result.status, output: `${result.stdout}${result.stderr}` };
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
}

function manifestWith(extra) {
  return JSON.stringify({ ...JSON.parse(repoFile("package.json")), ...extra }, null, 2);
}

function trivyReference() {
  return repoFile("scripts/image-scan.sh").match(/docker\.io\/aquasec\/trivy@sha256:[0-9a-f]{64}/)?.[0];
}

function checkoutReference() {
  return repoFile(".github/workflows/ci.yml").match(/actions\/checkout@[0-9a-f]{40}/)?.[0];
}

function goVersion() {
  return repoFile("Dockerfile").match(/golang:([0-9]+(?:\.[0-9]+)*)/)?.[1];
}

function bumpPatch(version) {
  const parts = version.split(".");
  parts[parts.length - 1] = String(Number(parts[parts.length - 1]) + 1);
  return parts.join(".");
}

// Replaces the nth `npm audit` command line, so the mutation survives a
// reordering of the flags and a reflow of the script's header.
function mutateAuditInvocation(audit, index, replacement) {
  const lines = audit.split("\n");
  let seen = 0;
  for (let i = 0; i < lines.length; i++) {
    if (!lines[i].trimStart().startsWith("npm audit")) continue;
    if (seen === index) {
      lines[i] = replacement;
      break;
    }
    seen++;
  }
  return lines.join("\n");
}

/**
 * The gate is the last line of defence for the dependency pins: a Dependabot
 * proposal that breaks one of them must not reach the merge. These specs run
 * the gate against a mutated copy of the repository, because the failures it
 * has to catch are exactly the ones the repository does not have.
 */
describe.skipIf(!HAS_NODE_MODULES)("check-dependabot gate", () => {
  it("passes on the repository as it stands", () => {
    const result = runCheck();
    expect(result.status).toBe(0);
  });

  it("rejects a configuration that parses to nothing", () => {
    const result = runCheck({ ".github/dependabot.yml": "# every line is a comment\n" });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/empty or not a mapping/);
  });

  it("rejects a configuration that does not parse", () => {
    const result = runCheck({ ".github/dependabot.yml": "version: 2\nupdates: [\n" });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/does not parse/);
  });

  it("rejects a configuration whose updates are not a list", () => {
    const result = runCheck({ ".github/dependabot.yml": "version: 2\nupdates: nonsense\n" });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/updates must be a list/);
  });

  it("rejects a configuration without update entries", () => {
    const result = runCheck({ ".github/dependabot.yml": "version: 2\nupdates: []\n" });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/no update entries/);
  });

  it("rejects an ignore rule that is not a list", () => {
    const config = [
      "version: 2",
      "updates:",
      "  - package-ecosystem: npm",
      '    directory: "/"',
      "    schedule:",
      '      interval: "weekly"',
      '      day: "monday"',
      '    versioning-strategy: "increase-if-necessary"',
      "    ignore: bootstrap",
    ].join("\n");
    const result = runCheck({ ".github/dependabot.yml": config });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/must ignore "bootstrap"/);
  });

  it("rejects a schedule that is not Monday", () => {
    const result = runCheck({ ".github/dependabot.yml": repoFile(".github/dependabot.yml").replace('day: "monday"', 'day: "friday"') });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/schedule\.day must be "monday"/);
  });

  it("rejects a second update entry for the same ecosystem", () => {
    const config = `${repoFile(".github/dependabot.yml")}\n  - package-ecosystem: "npm"\n    directory: "/"\n    schedule:\n      interval: "weekly"\n`;
    const result = runCheck({ ".github/dependabot.yml": config });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/more than one update entry/);
  });

  it("rejects an audit script whose production gate lost --omit=dev", () => {
    const audit = mutateAuditInvocation(repoFile("scripts/npm-audit.sh"), 0, "npm audit --audit-level=high");
    const result = runCheck({ "scripts/npm-audit.sh": audit });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/production-only gate/);
  });

  it("rejects an audit script whose informational audit became a gate", () => {
    const audit = mutateAuditInvocation(repoFile("scripts/npm-audit.sh"), 1, "npm audit --audit-level=high --omit=dev");
    const result = runCheck({ "scripts/npm-audit.sh": audit });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/informational dev-dependency audit/);
  });

  it("rejects an audit inlined into a workflow", () => {
    const probe = ["name: Probe", "on: push", "jobs:", "  probe:", "    runs-on: ubuntu-latest", "    steps:", "      - run: npm audit --audit-level=high --omit=dev"].join("\n");
    const result = runCheck({ ".github/workflows/probe.yml": probe });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/inlines an npm audit/);
  });

  it("rejects an audit inlined into a second script", () => {
    const script = ["#!/bin/bash", "npm audit --audit-level=high --omit=dev"].join("\n");
    const result = runCheck({ "scripts/other-audit.sh": script });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/inlines an npm audit/);
  });

  it("rejects an image-scan job that would scan a Dependabot pull request", () => {
    const workflow = repoFile(".github/workflows/ci.yml").replace("dependabot[bot]", "someone-else");
    const result = runCheck({ ".github/workflows/ci.yml": workflow });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/must skip Dependabot pull requests/);
  });

  it("does not read the trivy pin from a comment", () => {
    const script = `# TRIVY_IMAGE=docker.io/aquasec/trivy@sha256:${"0".repeat(64)}\n${repoFile("scripts/image-scan.sh")}`;
    const result = runCheck({ "scripts/image-scan.sh": script });
    expect(result.status).toBe(0);
  });

  it("does not read a base image from a comment", () => {
    const dockerfile = `# FROM alpine:3.20\n${repoFile("Dockerfile")}`;
    const result = runCheck({ "Dockerfile": dockerfile });
    expect(result.status).toBe(0);
  });

  it("rejects a configuration whose version is not 2", () => {
    const result = runCheck({ ".github/dependabot.yml": repoFile(".github/dependabot.yml").replace("version: 2", "version: 1") });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/version must be 2/);
  });

  it("rejects an update entry in the wrong directory", () => {
    const config = repoFile(".github/dependabot.yml").replace('directory: "/src"', 'directory: "/"');
    const result = runCheck({ ".github/dependabot.yml": config });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/gomod: directory must be "\/src"/);
  });

  it("rejects an npm entry that widens the version ranges", () => {
    const config = repoFile(".github/dependabot.yml").replace(
      'versioning-strategy: "increase-if-necessary"',
      'versioning-strategy: "widen"',
    );
    const result = runCheck({ ".github/dependabot.yml": config });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/versioning-strategy must be "increase-if-necessary"/);
  });

  it("rejects an ecosystem the checker does not know", () => {
    const config = `${repoFile(".github/dependabot.yml")}\n  - package-ecosystem: "cargo"\n    directory: "/"\n    schedule:\n      interval: "weekly"\n`;
    const result = runCheck({ ".github/dependabot.yml": config });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/unexpected ecosystem/);
  });

  it("rejects an unknown key in an update entry", () => {
    const config = repoFile(".github/dependabot.yml").replace('    directory: "/src"', '    directory: "/src"\n    totally-unknown-key: 1');
    const result = runCheck({ ".github/dependabot.yml": config });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/unknown key "totally-unknown-key"/);
  });

  it("rejects an update entry without groups", () => {
    const config = repoFile(".github/dependabot.yml").replace(/ {4}groups:\n[\s\S]*?(?= {2}- package-ecosystem)/, "");
    const result = runCheck({ ".github/dependabot.yml": config });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/must be grouped/);
  });

  it("rejects a bootstrap ignore that names versions", () => {
    const config = repoFile(".github/dependabot.yml").replace(
      '      - dependency-name: "bootstrap"',
      '      - dependency-name: "bootstrap"\n        versions: ["6.0.0-alpha.1"]',
    );
    const result = runCheck({ ".github/dependabot.yml": config });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/must name the dependency and nothing else/);
  });

  it("rejects a quoted audit invocation", () => {
    const probe = ["name: Probe", "on: push", "jobs:", "  probe:", "    runs-on: ubuntu-latest", "    steps:", '      - run: sh -c "npm audit --audit-level=high --omit=dev"'].join("\n");
    const result = runCheck({ ".github/workflows/probe.yml": probe });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/inlines an npm audit/);
  });

  it("does not mistake a block comment for an audit", () => {
    const script = ["#!/usr/bin/env node", "/*", " * The npm audit policy lives in scripts/npm-audit.sh.", " */", "export const done = true;"].join("\n");
    const result = runCheck({ "scripts/helper.mjs": script });
    expect(result.status).toBe(0);
  });

  it("rejects an image-scan job that scans Dependabot pull requests", () => {
    const workflow = repoFile(".github/workflows/ci.yml").replace("!= 'dependabot[bot]'", "== 'dependabot[bot]'");
    const result = runCheck({ ".github/workflows/ci.yml": workflow });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/must skip Dependabot pull requests/);
  });

  it("rejects a manifest without bootstrap", () => {
    const manifest = JSON.parse(repoFile("package.json"));
    delete manifest.dependencies.bootstrap;
    const result = runCheck({ "package.json": JSON.stringify(manifest, null, 2) });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/no longer lists bootstrap/);
  });

  it("rejects a CI without the ops-check job", () => {
    const workflow = repoFile(".github/workflows/ci.yml").replace(/ {2}ops-check:\n(?: {4}.*\n| {6}.*\n| {8}.*\n)+/, "");
    const result = runCheck({ ".github/workflows/ci.yml": workflow });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/no ops-check job/);
  });

  it("rejects an ops-check job that runs something else", () => {
    const workflow = repoFile(".github/workflows/ci.yml").replace("node scripts/check-dependabot.mjs", "true");
    const result = runCheck({ ".github/workflows/ci.yml": workflow });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/does not run scripts\/check-dependabot\.mjs/);
  });

  it("rejects an ops-check job that can be skipped", () => {
    const workflow = repoFile(".github/workflows/ci.yml").replace("  ops-check:\n", "  ops-check:\n    if: false\n");
    const result = runCheck({ ".github/workflows/ci.yml": workflow });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/must not carry a condition/);
  });

  it("rejects an image-scan job that compares the wrong field", () => {
    const workflow = repoFile(".github/workflows/ci.yml").replace(
      "github.event.pull_request.user.login",
      "github.event.pull_request.head.ref",
    );
    const result = runCheck({ ".github/workflows/ci.yml": workflow });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/must skip Dependabot pull requests/);
  });

  it("rejects an invalid group name", () => {
    const config = repoFile(".github/dependabot.yml").replace("      go-major:", "      go major:");
    const result = runCheck({ ".github/dependabot.yml": config });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/not a valid group name/);
  });

  it("rejects an update entry with both directory and directories", () => {
    const config = repoFile(".github/dependabot.yml").replace('    directory: "/src"', '    directory: "/src"\n    directories: ["/src"]');
    const result = runCheck({ ".github/dependabot.yml": config });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/cannot both be set/);
  });

  it("rejects a quoted audit invocation with combined shell flags", () => {
    const probe = ["name: Probe", "on: push", "jobs:", "  probe:", "    runs-on: ubuntu-latest", "    steps:", '      - run: bash -lc "npm audit --audit-level=high --omit=dev"'].join("\n");
    const result = runCheck({ ".github/workflows/probe.yml": probe });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/inlines an npm audit/);
  });

  it("rejects two trivy pins that differ", () => {
    const reference = trivyReference();
    const result = runCheck({
      "Taskfile.yml": repoFile("Taskfile.yml").replace(reference, "docker.io/aquasec/trivy@sha256:" + "0".repeat(64)),
    });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/not the same everywhere/);
  });

  it("rejects an audit inlined into a Taskfile task", () => {
    const taskfile = repoFile("Taskfile.yml").replace("bash scripts/npm-audit.sh", "npm audit --audit-level=high --omit=dev");
    const result = runCheck({ "Taskfile.yml": taskfile });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/inlines an npm audit/);
  });

  it("rejects a workflow that names a second Go version", () => {
    const workflow = repoFile(".github/workflows/ci.yml").replace("GO_VERSION: 1.26.10", "GO_VERSION: 1.26.11");
    const result = runCheck({ ".github/workflows/ci.yml": workflow });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/go-toolchain/);
  });

  it("rejects a manifest the Node loader refuses", () => {
    const result = runCheck({ "package.json": "{ not json" });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/does not parse|ERR_INVALID_PACKAGE_CONFIG/);
  });

  it("rejects a missing manifest", () => {
    const result = runCheck({ "package.json": null });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/package\.json is missing/);
  });

  it("rejects a nested braces override", () => {
    const result = runCheck({ "package.json": manifestWith({ overrides: { micromatch: { braces: "3.0.4" } } }) });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/braces/);
  });

  it("rejects a script that would force the audit fix", () => {
    const result = runCheck({ "package.json": manifestWith({ scripts: { "fix:audit": "npm audit fix --force" } }) });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/npm audit fix --force/);
  });

  it("rejects a trivy pin that is a mutable tag", () => {
    const reference = trivyReference();
    const result = runCheck({
      "Taskfile.yml": repoFile("Taskfile.yml").replace(reference, "docker.io/aquasec/trivy:0.75.0"),
      "scripts/image-scan.sh": repoFile("scripts/image-scan.sh").replace(reference, "docker.io/aquasec/trivy:0.75.0"),
    });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/pinned by digest/);
  });

  it("rejects an action that lost its commit SHA", () => {
    const workflow = repoFile(".github/workflows/ci.yml").replace(checkoutReference(), "actions/checkout@v5");
    const result = runCheck({ ".github/workflows/ci.yml": workflow });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/commit SHA/);
  });

  it("rejects an unpinned action in a composite action", () => {
    const action = ["name: npm audit", "description: Audit", "runs:", "  using: composite", "  steps:", "    - uses: actions/checkout@v5"].join("\n");
    const result = runCheck({ ".github/actions/npm-audit/action.yml": action });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/commit SHA/);
  });

  it("does not mistake a run block for an action reference", () => {
    const probe = ["name: Probe", "on: push", "jobs:", "  probe:", "    runs-on: ubuntu-latest", "    steps:", `      - run: 'echo "uses: owner/action@v1"'`].join("\n");
    const result = runCheck({ ".github/workflows/probe.yml": probe });
    expect(result.status).toBe(0);
  });

  it("does not mistake a step input for an action reference", () => {
    const probe = ["name: Probe", "on: push", "jobs:", "  probe:", "    runs-on: ubuntu-latest", "    steps:", "      - uses: actions/checkout@" + "0".repeat(40), "        with:", "          uses: not-an-action"].join("\n");
    const result = runCheck({ ".github/workflows/probe.yml": probe });
    expect(result.status).toBe(0);
  });

  it("accepts a container image reference", () => {
    const probe = ["name: Probe", "on: push", "jobs:", "  probe:", "    runs-on: ubuntu-latest", "    steps:", "      - uses: docker://alpine:3.23"].join("\n");
    const result = runCheck({ ".github/workflows/probe.yml": probe });
    expect(result.status).toBe(0);
  });

  it("rejects a Go toolchain that drifted in one image", () => {
    const version = goVersion();
    const result = runCheck({ "Dockerfile": repoFile("Dockerfile").replace(`golang:${version}`, `golang:${bumpPatch(version)}`) });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/go-toolchain/);
  });

  it("rejects a base image that drifted between the Dockerfiles", () => {
    const version = repoFile("Dockerfile").match(/alpine:([0-9]+(?:\.[0-9]+)*)/)?.[1];
    const result = runCheck({ "Dockerfile": repoFile("Dockerfile").replace(`alpine:${version}`, `alpine:${bumpPatch(version)}`) });
    expect(result.status).toBe(1);
    expect(result.output).toMatch(/base-image/);
  });
});
