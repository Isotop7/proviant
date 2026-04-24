#!/bin/bash
# Conventional Commit Validator for git-cliff compatibility
# Run: ./scripts/conventional-commit-validator.sh [commit-range]

set -e

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

VALID_TYPES="feat|fix|refactor|change|update|perf|deprecat|remove|delete|security|doc|test|ci|chore|build|style|implement|add|hotfix|bug"

validate_message() {
    local msg=$(echo "$1" | head -n1 | tr -d '\r' | xargs)

    if [ -z "$msg" ]; then
        echo "❌ INVALID: Empty commit message"
        return 1
    fi

    if echo "$msg" | grep -qE "^($VALID_TYPES)(\(.*\))?!?: .+"; then
        echo "✅ VALID: $msg"
        return 0
    else
        echo "❌ INVALID: $msg"
        echo "   Format: <type>[(<scope>)]!: <description>"
        echo "   Types: feat, fix, docs, chore, refactor, test, ci, build, style, etc."
        return 1
    fi
}

check_range() {
    local range="${1:-HEAD~9..HEAD}"
    local commits
    commits=$(git log --format="%h %s" "$range" 2>/dev/null || true)

    if [ -z "$commits" ]; then
        echo "No commits in range '$range'"
        return 0
    fi

    local invalid=0 count=0
    echo "Checking commits in range: $range"
    echo "----------------------------------------"

    while IFS= read -r line; do
        count=$((count + 1))
        hash=$(echo "$line" | awk '{print $1}')
        msg=$(echo "$line" | cut -d' ' -f2-)

        if ! validate_message "$msg" > /dev/null 2>&1; then
            echo "  ❌ $hash $msg"
            invalid=$((invalid + 1))
        else
            echo "  ✅ $hash $msg"
        fi
    done <<< "$commits"

    echo "----------------------------------------"
    echo "Checked: $count | Invalid: $invalid"
    return $invalid
}

case "${1:-check}" in
    check)
        shift
        check_range "$1"
        ;;
    validate)
        shift
        if [ -z "$1" ]; then
            echo "Usage: $0 validate <commit_message>"
            exit 1
        fi
        validate_message "$1"
        ;;
    *)
        echo "Usage: $0 [check|validate] [args]"
        echo "  check [range]  - Validate commits (default: last 10)"
        echo "  validate <msg> - Validate single message"
        exit 1
        ;;
esac
