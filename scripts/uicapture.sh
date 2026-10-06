#!/usr/bin/env bash
# Thin wrapper around the ui-capture skill's orchestrator.
#
# The skill lives under .kilo/skills/ui-capture/ and owns all the logic
# (preflight, Go build, demo seed, temp server, container launch, teardown).
# This script only resolves the repo root and forwards every argument, so
# `task uicapture -- --preset dashboard` and
# `node .kilo/skills/ui-capture/scripts/uicapture.mjs --preset dashboard`
# are the same run.
#
# Exit codes are passed through unchanged (see the skill's SKILL.md).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
UICAPTURE="$REPO_ROOT/.kilo/skills/ui-capture/scripts/uicapture.mjs"

if [ ! -f "$UICAPTURE" ]; then
  echo "uicapture: $UICAPTURE not found." >&2
  echo "  The ui-capture skill lives under .kilo/skills/ — is it checked out?" >&2
  exit 1
fi

exec node "$UICAPTURE" "$@"
