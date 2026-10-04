#!/bin/bash
# Audit npm dependencies.
#
# Production dependencies gate the build. Dev-dependency findings are reported
# for visibility but never fail the run, because the only dev advisory in this
# tree (GHSA-vfj7-8cjw-p6xm in `braces`, via stylelint) has no upstream fix and
# is unreachable at runtime.
#
# This script is the single source of truth for that policy. It is invoked by
# both CI workflows (via the composite action .github/actions/npm-audit) and by
# `task vuln-npm`, so the gate can never drift between them.
#
# `set -e` is deliberately omitted: both audits are expected to exit non-zero,
# and the gate's status must be captured rather than abort the script.
#
# Exit status: the production audit's status (0 when only dev findings exist).
#
# See AGENTS.md "Dependency Auditing" for the full rationale and re-check trigger.

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
  echo "::warning::npm audit reported dev-dependency vulnerabilities outside the production gate. Non-blocking by design — see AGENTS.md#dependency-auditing"
fi

exit "$gate_status"