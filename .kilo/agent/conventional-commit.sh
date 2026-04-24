#!/bin/bash
set -e

# Conventional Commit Helper - validates and generates commit messages
# Compatible with git-cliff configuration in cliff.toml

PROJECT_ROOT="/home/hendrik/dev/proviant"
VALID_TYPES="feat|fix|refactor|change|update|perf|deprecat|remove|delete|security|doc|test|ci|chore|build|style|implement|add|hotfix|bug"

show_help() {
    cat << EOF
Conventional Commit Helper

Usage: kilo conventional-commit <command> [options]

Commands:
    check [range]     Validate commit messages in range (default: unpushed commits)
    suggest           Generate commit message from all uncommitted changes
    fix [--amend]     Validate/fix last commit message
    validate [msg]    Validate a single commit message string

Examples:
    kilo conventional-commit check
    kilo conventional-commit check HEAD~5..HEAD
    kilo conventional-commit suggest
    kilo conventional-commit fix --amend

EOF
}

get_unpushed_commits() {
    git rev-list --count @{upstream}..HEAD 2>/dev/null || git rev-list --count HEAD~0..HEAD
}

get_commits_in_range() {
    if [ -z "$1" ]; then
        # Default: all commits not yet pushed
        local unpushed=$(get_unpushed_commits)
        if [ "$unpushed" -eq 0 ]; then
            echo "No unpushed commits."
            return 0
        fi
        git log --format="%h %s" @{upstream}..HEAD 2>/dev/null || git log --format="%h %s" HEAD~0..HEAD
    else
        git log --format="%h %s" "$1"
    fi
}

validate_message() {
    local msg="$1"
    local short_msg=$(echo "$msg" | head -n1 | tr -d '\r')

    # Empty message
    if [ -z "$short_msg" ]; then
        echo "INVALID: Empty commit message"
        return 1
    fi

    # Check conventional commit pattern: type(optional scope): description
    # Matches: feat: fix: docs: chore: etc.
    if echo "$short_msg" | grep -qE "^($VALID_TYPES)(\(.*\))?!?: .+"; then
        echo "VALID: $short_msg"
        return 0
    else
        echo "INVALID: $short_msg"
        echo "  Expected format: <type>[(<scope>)]!: <description>"
        echo "  Types: $(echo $VALID_TYPES | tr '|' ', ')"
        return 1
    fi
}

check_commits() {
    local range="${1:-}"
    local commits=$(get_commits_in_range "$range")
    local invalid=0
    local count=0

    echo "Checking commit messages..."
    if [ -z "$range" ]; then
        echo "(unpushed commits)"
    else
        echo "(range: $range)"
    fi
    echo ""

    while IFS= read -r line; do
        count=$((count + 1))
        hash=$(echo "$line" | awk '{print $1}')
        msg=$(echo "$line" | cut -d' ' -f2-)

        if ! validate_message "$msg" > /dev/null 2>&1; then
            echo "  [INVALID] $hash $msg"
            invalid=$((invalid + 1))
        else
            echo "  [OK]      $hash $msg"
        fi
    done <<< "$commits"

    echo ""
    echo "Checked $count commits, $invalid invalid."
    return $invalid
}

suggest_message() {
    # Collect all uncommitted changes: staged, unstaged tracked, and untracked
    local status_output
    status_output=$(git status --porcelain)
    if [ -z "$status_output" ]; then
        echo "No uncommitted changes. Make changes and stage or leave them unstaged."
        return 1
    fi

    echo "Analyzing all uncommitted changes..."
    echo ""
    git status -s
    echo ""

    # Simple heuristics for type detection
    local type="chore"
    local scope=""
    local description=""

    # Categorize changes from porcelain output
    # Format: XY PATH (X=index status, Y=working tree status)
    local added=0 modified=0 deleted=0 untracked=0

    while IFS= read -r line; do
        # Skip empty lines
        [ -z "$line" ] && continue

        # Extract status codes
        local x="${line:0:1}"
        local y="${line:1:1}"

        # Tracked added (A in index)
        if [ "$x" = "A" ]; then
            added=$((added + 1))
        fi
        # Tracked modified (M in index or working tree)
        if [ "$x" = "M" ] || [ "$y" = "M" ]; then
            modified=$((modified + 1))
        fi
        # Tracked deleted (D in index or working tree)
        if [ "$x" = "D" ] || [ "$y" = "D" ]; then
            deleted=$((deleted + 1))
        fi
        # Untracked files (?? in working tree)
        if [ "$x" = "?" ] && [ "$y" = "?" ]; then
            untracked=$((untracked + 1))
        fi
    done <<< "$status_output"

    # Show counts
    echo "Changes summary:"
    echo "  Added (staged):      $added"
    echo "  Modified:            $modified"
    echo "  Deleted:             $deleted"
    echo "  Untracked:           $untracked"
    echo ""

    # Detect scope from file paths
    local all_files
    all_files=$(git status --porcelain | awk '{print $2}')
    if echo "$all_files" | grep -q '^src/'; then
        scope="backend"
    elif echo "$all_files" | grep -q '^src/templates/'; then
        scope="templates"
    elif echo "$all_files" | grep -q '^src/assets/'; then
        scope="assets"
    elif echo "$all_files" | grep -q '^docs/'; then
        scope="docs"
    fi

    # Detect scope from directory structure
    if echo "$changes" | grep -q 'src/'; then
        scope="backend"
    elif echo "$changes" | grep -q 'src/templates/'; then
        scope="templates"
    elif echo "$changes" | grep -q 'src/assets/'; then
        scope="assets"
    elif echo "$changes" | grep -q 'docs/'; then
        scope="docs"
    fi

    # Determine type based on total change counts (adds priority: untracked > added > modified > deleted)
    local total_added=$((added + untracked))

    if [ "$total_added" -gt 0 ] && [ "$modified" -eq 0 ] && [ "$deleted" -eq 0 ]; then
        # Pure addition of files
        if echo "$all_files" | grep -qE '\.md$|README'; then
            type="docs"
            description="update documentation"
        else
            type="feat"
            description="add new files"
        fi
    elif [ "$modified" -gt 0 ]; then
        # Modifications present
        if echo "$all_files" | grep -qE '\.go$|\.js$|\.css$|\.scss$|\.ts$'; then
            if echo "$all_files" | grep -qE 'test|_test\.'; then
                type="test"
                description="update tests"
            else
                type="refactor"
                description="refactor code"
            fi
        elif echo "$all_files" | grep -qE '\.md$|README'; then
            type="docs"
            description="update documentation"
        else
            type="chore"
            description="update files"
        fi
    elif [ "$deleted" -gt 0 ]; then
        type="remove"
        description="remove files"
    else
        type="chore"
        description="update project"
    fi

    # Build suggested message
    local suggestion=""
    if [ -n "$scope" ]; then
        suggestion="$type($scope): $description"
    else
        suggestion="$type: $description"
    fi

    echo "Suggested commit message:"
    echo "  $suggestion"
    echo ""
    echo "To use: git commit -m \"$suggestion\""
    echo "Or: git commit -m \"$(echo $suggestion)\" --edit to edit before committing"
}

fix_last_commit() {
    local amend=0
    if [ "$1" = "--amend" ]; then
        amend=1
    fi

    local last_msg=$(git log -1 --pretty=%B)
    echo "Current last commit message:"
    echo "  $last_msg"
    echo ""

    if validate_message "$last_msg"; then
        echo "Last commit message already valid."
        return 0
    fi

    if [ $amend -eq 1 ]; then
        echo "Generating suggestion based on last commit changes..."
        # Get the changes from the last commit
        git show HEAD --stat | tail -n +2 | head -n 20
        echo ""
        echo "Run these commands to fix:"
        echo "  1. git reset --soft HEAD~1"
        echo "  2. kilo conventional-commit suggest"
        echo "  3. git commit --amend -m \"<your validated message>\""
        return 1
    fi

    echo "To fix: amend the commit with a valid conventional commit message."
    echo "  git commit --amend -m \"feat: your description\""
}

# Main command dispatch
case "$1" in
    check)
        check_commits "$2"
        ;;
    suggest)
        suggest_message
        ;;
    fix)
        shift
        fix_last_commit "$1"
        ;;
    validate)
        shift
        if [ -z "$1" ]; then
            echo "Usage: kilo conventional-commit validate <commit_message>"
            exit 1
        fi
        validate_message "$1"
        ;;
    --help|-h)
        show_help
        ;;
    *)
        echo "Unknown command: $1"
        show_help
        exit 1
        ;;
esac
