#!/bin/bash
# Scan a container image for vulnerabilities using Trivy.
#
# Policy: gate the build on HIGH and CRITICAL severities only, and only on
# vulnerabilities with an upstream fix (--ignore-unfixed). Unfixed findings are
# still written to trivy-report.json for visibility but never fail the run:
# there is no action a maintainer can take for them, and blocking on them
# stalls CI indefinitely. Entries in the base image (alpine, tesseract) are
# the typical source; fix them by bumping the base image, not by silencing
# the scanner.
#
# The report is the whole point of the gate, so it must survive every exit
# path — including a scan that fails for non-vulnerability reasons (DB
# download failure, unreadable image). Trivy exits non-zero for both cases,
# so neither the exit code nor the report alone can classify the run: the exit
# code says "something happened", the report says "what was found", and only
# the pair separates a finding from a scanner outage. Both are consulted —
# see classify().
#
# This script is the single source of truth for that policy. It is invoked by
# both CI workflows (ci.yml image-scan job, release.yml docker-publish job) and
# by `task vuln-image`, so the gate can never drift between them.
#
# Usage: scripts/image-scan.sh <image-ref-or-tarball>
#   image-ref    e.g. proviant:scan — scanned via the local trivy binary
#                (GitHub runners ship trivy; CI always uses this path)
#   tarball      path to a `docker/podman save` archive — required when trivy
#                must run inside a container (task vuln-image), because the
#                container has no access to the host runtime socket
#
# Trivy is resolved in this order:
#   1. `trivy` on PATH (CI runners, hosts with trivy installed)
#   2. the image referenced by $TRIVY_IMAGE (default: digest-pinned, see
#      AGENTS.md "Container Image Scanning" for the pin and bump procedure)
#
# Exit status: 0 when no fixable HIGH/CRITICAL vulnerability is found, 1 otherwise.

set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 <image-ref-or-tarball>" >&2
  exit 2
fi

TARGET="$1"
REPORT_NAME="trivy-report.json"

# Default TRIVY_IMAGE: sync with Taskfile.yml var TRIVY_IMAGE — bump both together when updating
# See: AGENTS.md "Container Image Scanning" for bump procedure
TRIVY_IMAGE="${TRIVY_IMAGE:-docker.io/aquasec/trivy@sha256:af6acf9a6b85dfe389a1941505c0ce9efef52a4719635e1a962f022a3d855daa}"
# Vulnerability DB cache, honored by BOTH paths: the local binary receives it via
# --cache-dir, the container path bind-mounts it. All CI workflows cache this
# directory with actions/cache so the DB download is skipped on cache hit.
TRIVY_CACHE_DIR="${TRIVY_CACHE_DIR:-$(git rev-parse --show-toplevel 2>/dev/null || echo "$PWD")/.cache/trivy}"
if ! mkdir -p "$TRIVY_CACHE_DIR"; then
  echo "Cannot create trivy cache dir: $TRIVY_CACHE_DIR" >&2
  exit 2
fi

# --scanners vuln: trivy image defaults to "vuln,secret". Secret findings carry
# HIGH/CRITICAL severities too and would trip --exit-code 1, but this gate is
# scoped to OS packages (see AGENTS.md). Pin the intent instead of relying on
# the default.
# shellcheck disable=SC2054 # HIGH,CRITICAL is one comma-separated trivy argument, not two
SEVERITY_FLAGS=(--scanners vuln --severity HIGH,CRITICAL --ignore-unfixed --exit-code 1)

# Echoes the number of gated findings. Exit status 0 = report is usable,
# 1 = missing or not a completed scan.
count_findings() {
  [ -f "$REPORT_NAME" ] || return 1
  if command -v jq >/dev/null 2>&1; then
    # A completed scan always writes a Results array. trivy leaves "{}" or a
    # truncated document behind when it aborts early (DB download, image
    # pull), and that must not read as "zero findings".
    jq -e '(.Results | type) == "array"' "$REPORT_NAME" >/dev/null 2>&1 || return 1
    jq '[.Results[]?.Vulnerabilities[]?] | length' "$REPORT_NAME" 2>/dev/null | tr -d '[:space:]'
  else
    # jq is not guaranteed on dev machines. Counting VulnerabilityID keys is
    # an approximation, but it is only used to tell "findings" from "no
    # findings" — the report stays the source of truth either way.
    grep -q '"Results"' "$REPORT_NAME" || return 1
    # `|| true` is required: without jq a clean report matches nothing, grep
    # exits 1 and pipefail would turn "no findings" into "unusable report".
    { grep -o '"VulnerabilityID"' "$REPORT_NAME" || true; } | wc -l | tr -d '[:space:]'
  fi
}

# Trivy exits non-zero both for findings and for operational failures (DB
# download failure, unreadable image, killed container), so the exit code
# alone cannot classify the run. The report is preserved on every path; this
# decides which message the maintainer gets, so a scanner blip is never
# presented as a security verdict.
classify() {
  local rc="$1" findings
  if ! findings=$(count_findings); then
    echo "Scan failed and produced no usable $REPORT_NAME (trivy exit $rc)." >&2
    echo "This is a scanner/infrastructure error, not a vulnerability verdict." >&2
    exit 1
  fi
  # The case that must never pass: trivy exited non-zero but the report lists
  # nothing. That is an aborted scan (DB download, registry auth, OOM), not a
  # clean image — reporting it as "no vulnerabilities found" turns a scanner
  # outage into a green security gate on main and on every release.
  if [ "$rc" -ne 0 ] && [ "$findings" -eq 0 ]; then
    echo "trivy exited $rc but $REPORT_NAME lists no findings." >&2
    echo "The scan did not complete — treating this as an infrastructure error, not a clean result." >&2
    exit 1
  fi
  if [ "$findings" -gt 0 ]; then
    echo "Fixable HIGH/CRITICAL vulnerability found ($findings). See $REPORT_NAME for details." >&2
    exit 1
  fi
  echo "No fixable HIGH/CRITICAL vulnerability found. See $REPORT_NAME for details."
  exit 0
}

scan_local() {
  local rc=0
  if [ -f "$TARGET" ]; then
    trivy image --input "$TARGET" \
      --cache-dir "$TRIVY_CACHE_DIR" \
      "${SEVERITY_FLAGS[@]}" \
      --format json --output "$REPORT_NAME" || rc=$?
  else
    trivy image "$TARGET" \
      --cache-dir "$TRIVY_CACHE_DIR" \
      "${SEVERITY_FLAGS[@]}" \
      --format json --output "$REPORT_NAME" || rc=$?
  fi
  classify "$rc"
}

scan_in_container() {
  local rc=0
  if [ ! -f "$TARGET" ]; then
    echo "Container-based scanning requires an image tarball (docker/podman save archive), got: $TARGET" >&2
    exit 2
  fi
  # Podman wants the :Z SELinux relabel on bind mounts; docker rejects the
  # option entirely, so it is only added when the runtime is podman.
  Z=""
  case "$CONTAINER_RUNTIME" in
    *podman*) Z=":Z" ;;
  esac
  # Use a temp dir for output to avoid mounting the whole repo read-write
  # into the scan container and to avoid root-owned report files.
  OUT_DIR="$(mktemp -d)"
  "$CONTAINER_RUNTIME" run --rm \
    -v "$(dirname "$TARGET"):/input$Z,ro" \
    -v "$OUT_DIR:/out$Z" \
    -v "$TRIVY_CACHE_DIR:/root/.cache/trivy$Z" \
    "$TRIVY_IMAGE" image \
    --input "/input/$(basename "$TARGET")" \
    "${SEVERITY_FLAGS[@]}" \
    --format json --output "/out/$REPORT_NAME" || rc=$?
  # Copy the report out before classifying. On a vulnerability finding trivy
  # exits 1 and the report is exactly what the maintainer needs — the previous
  # `|| rm -rf "$OUT_DIR"` deleted it in precisely that case.
  if [ -f "$OUT_DIR/$REPORT_NAME" ]; then
    cp "$OUT_DIR/$REPORT_NAME" "$REPORT_NAME"
  fi
  rm -rf "$OUT_DIR"
  classify "$rc"
}

if command -v trivy >/dev/null 2>&1; then
  # The local binary is not digest-pinned — log its version so the CI record
  # shows exactly which scanner ran (the digest pin only covers the container path).
  # A runner image that stops shipping trivy would silently switch CI to the
  # container path, so mirror this into the step summary when there is one.
  trivy_version="$(trivy --version 2>&1 | head -n1 || true)"
  echo "Using local trivy: $trivy_version"
  if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
    {
      echo "### Container image scan"
      echo "- scanner: \`$trivy_version\` (local binary — **not** digest-pinned)"
      echo "- target: \`$TARGET\`"
    } >> "$GITHUB_STEP_SUMMARY"
  fi
  scan_local
else
  if [ -z "${CONTAINER_RUNTIME:-}" ]; then
    CONTAINER_RUNTIME="$(command -v podman || command -v docker || true)"
  fi
  if [ -n "$CONTAINER_RUNTIME" ]; then
    echo "Using pinned trivy image: $TRIVY_IMAGE"
    scan_in_container
  else
    echo "trivy not found and no container runtime available." >&2
    echo "Install trivy (https://trivy.dev) or podman/docker to scan via the pinned trivy container image." >&2
    exit 2
  fi
fi
