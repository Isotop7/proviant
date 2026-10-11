#!/bin/bash
# Audit npm dependencies.
#
# Production dependencies gate the build (`npm audit --audit-level=high
# --omit=dev`). Dev-dependency findings are reported for visibility but never
# fail the run: the only dev advisory in this tree has no upstream fix and is
# unreachable at runtime.
#
#   Advisory: GHSA-vfj7-8cjw-p6xm (CVE-2026-93687, high) affects `braces`
#     through 3.0.3, and 3.0.3 is the latest published version — so `npm audit
#     fix` can never resolve it.
#   Reached via: `stylelint` (devDependency) -> `micromatch ^4.0.8` /
#     `fast-glob` / `globby` -> `braces`. Upgrading stylelint does not help
#     (stylelint 17.16.0 still depends on micromatch ^4.0.8).
#   Unreachable: these packages run only during `task lint-css`, on
#     repo-controlled globs — no untrusted input reaches `braces`, so the DoS
#     precondition does not exist.
#
# Re-check trigger: once a `braces` release greater than 3.0.3 ships, drop
#   `--omit=dev` and re-enable the full gate.
#   Upstream: https://github.com/micromatch/braces/issues/70
#
# Never reintroduce `npm audit fix --force` or a `braces` `overrides` entry —
# no fixed version exists, and neither clears the advisory.
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

if [ "$info_status" -ne 0 ] && [ "$gate_status" -eq 0 ]; then
  echo "::warning::npm audit reported dev-dependency vulnerabilities outside the production gate. Non-blocking by design — see the header of scripts/npm-audit.sh"
fi

exit "$gate_status"