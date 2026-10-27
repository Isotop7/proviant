#!/bin/bash
# Report (or apply) the Go modernizations the code is missing.
#
# `go fix` ships the modernizers Go 1.26 knows: rangeint, minmax,
# slicescontains, slicessort, stringscut, stringsseq, mapsloop, waitgroup,
# testingcontext, newexpr, any.
#
# Two traps this script exists to close:
#
#   1. `go fix -diff` writes the patch to stdout and load errors to stderr.
#      A package that fails to typecheck aborts the WHOLE run and still yields
#      an empty patch — so a tree that does not compile reads as "no
#      modernizations pending" and the gate passes on code it never checked.
#      A patch is never empty, so an empty stdout with a non-zero status is
#      the signal that the tree could not be analyzed. The apply mode uses
#      the exit status of `go fix` directly: it exits 0 on success, also when
#      it applied fixes, and non-zero when it could not.
#
#   2. One pass can expose the next fix only after the previous one landed
#      (a nested min/max collapses only once the inner one is in place), so the
#      apply mode iterates until the patch stays empty.
#
# The `b.N` benchmark loop is deliberately NOT converted: the rangeint
# analyzer skips a `b.N` limit on purpose (it is reserved for the bloop
# analyzer, which `go fix` does not ship). Those loops use `for b.Loop()`.
#
# The check needs a compiling tree, and `src/assets/css` is gitignored — on a
# fresh clone the `assets` package fails to embed until the CSS is built.
#
# This script is the single source of truth for the policy: CI invokes it and
# so do `task modernize` and `task modernize-fix`, so the gate can never drift
# between them. Never inline `go fix` into a workflow or the Taskfile — edit
# this script.
#
# `set -e` is deliberately omitted: `go fix` exits non-zero when the patch is
# non-empty, and that status is what the gate reports on.
#
# Usage: scripts/check-modernize.sh [--fix]
#
# Exit status: 0 when the tree is modern, 1 when modernizations are pending
# (or the tree could not be analyzed), 2 on an internal error.

set -uo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT/src" || exit 2

fix=false
if [ "${1:-}" = "--fix" ]; then
	fix=true
elif [ $# -gt 0 ]; then
	echo "usage: $0 [--fix]" >&2
	exit 2
fi

err_file=""
diff_file=""
trap 'rm -f "$err_file" "$diff_file"' EXIT
err_file="$(mktemp)"
diff_file="$(mktemp)"

# Collect the pending patch. Returns 0 when there is none, 1 when there is
# one, 2 when `go fix` could not analyze the tree.
collect() {
	go fix -diff ./... >"$diff_file" 2>"$err_file"
	status=$?
	if [ -s "$diff_file" ]; then
		return 1
	fi
	if [ "$status" -ne 0 ]; then
		return 2
	fi
	return 0
}

report_unanalyzable() {
	echo "go fix could not analyze the tree:" >&2
	cat "$err_file" >&2
	echo >&2
	echo "The whole module has to compile for the modernization check to run." >&2
	echo "On a fresh clone the compiled assets are missing — run 'task css' (or 'task init') first." >&2
}

if [ "$fix" = true ]; then
	for _ in $(seq 1 10); do
		collect
		status=$?
		[ "$status" -eq 2 ] && report_unanalyzable && exit 1
		[ "$status" -eq 0 ] && break
		go fix ./... >/dev/null 2>"$err_file"
		apply_status=$?
		if [ "$apply_status" -ne 0 ]; then
			echo "go fix could not apply the modernizations:" >&2
			cat "$err_file" >&2
			exit 1
		fi
	done
	collect
	status=$?
	[ "$status" -eq 2 ] && report_unanalyzable && exit 1
	if [ "$status" -eq 1 ]; then
		[ -s "$err_file" ] && cat "$err_file" >&2
		cat "$diff_file"
		echo "Modernizations remain — re-run the task." >&2
		exit 1
	fi
	go fmt ./... || exit 1
	echo "Modernizations applied."
	exit 0
fi

collect
status=$?
[ "$status" -eq 2 ] && report_unanalyzable && exit 1
if [ "$status" -eq 1 ]; then
	[ -s "$err_file" ] && cat "$err_file" >&2
	cat "$diff_file"
	echo "Modernizations pending — run 'task modernize-fix' to apply them." >&2
	exit 1
fi
echo "No modernizations pending."
