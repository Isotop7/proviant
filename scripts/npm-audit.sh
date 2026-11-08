#!/bin/bash
# Audit npm dependencies.
#
# Production dependencies gate the build (`npm audit --audit-level=high
# --omit=dev`). Dev-dependency findings are reported for visibility but never
# fail the run: a dev advisory is either closed by an `overrides` entry in
# package.json or documented below as unreachable at runtime.
#
# Dev advisories closed by an override (package.json):
#
#   GHSA-3g3m-2fjr-6pw4 (moderate) — `glob` up to 14.0.1 ignores an aborted
#     traversal and keeps touching the filesystem after the timeout.
#     Fixed in 14.0.2; `htmlhint` (devDependency) asks for `glob ^13.0.6`,
#     so the tree is pinned to `^14.0.4`.
#   GHSA-g3hv-75wg-m249 (moderate) — `minimatch` 7.0.0 to 11.0.0 expands a
#     pattern into an exponential number of parts (denial of service).
#     Fixed in 11.0.1; `eslint` asks for `minimatch ^10.2.6`, so the tree is
#     pinned to `^11.0.5`. The pin is scoped to `minimatch@>=7.0.0` because
#     `npm-run-all` needs the 3.x API.
#
# Withdrawn upstream:
#
#   GHSA-vfj7-8cjw-p6xm (CVE-2026-93687, high) — the `braces` advisory this
#     script used to carry was withdrawn on 2026-10-23 as "not a practically
#     exploitable vulnerability", so npm audit stopped reporting it. There is
#     nothing to upgrade and no override to add.
#
# Never reintroduce `npm audit fix --force`: it resolves a dev advisory by
# downgrading the linters (`eslint` 10 to 9, `htmlhint` 1.9 to 1.7) and
# breaks the toolchain. `task check-dependabot` fails on it.
#
# This script is the single source of truth for the policy: both CI workflows
# invoke it (via the composite action .github/actions/npm-audit) and so does
# `task vuln-npm`, so the gate can never drift between them. Never inline
# `npm audit` into a workflow or the Taskfile — edit this script.
#
# `set -e` is deliberately omitted: both audits are expected to exit non-zero,
# and the gate's status must be captured rather than abort the script.
#
# Exit status: the production audit's status (0 when only dev findings exist).

set -uo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "==> Auditing production dependencies (gating)"
npm audit --audit-level=high --omit=dev
gate_status=$?

echo "==> Auditing all dependencies (informational)"
npm audit --audit-level=high
info_status=$?

if [[ "$info_status" -ne 0 ]] && [[ "$gate_status" -eq 0 ]]; then
  echo "::warning::npm audit reported dev-dependency vulnerabilities outside the production gate. Non-blocking by design — see the header of scripts/npm-audit.sh"
fi

exit "$gate_status"