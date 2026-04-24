# Conventional Commit Helper

A tool to validate and generate conventional commit messages for git-cliff compatibility.

## Usage

```bash
# Validate the last N commits (default: all unpushed)
kilo conventional-commit check

# Validate specific commit range
kilo conventional-commit check HEAD~5..HEAD

# Validate staged changes and suggest commit message
kilo conventional-commit suggest

# Interactive mode: validate and fix last commit message
kilo conventional-commit fix

# Show help
kilo conventional-commit --help
```

## Examples

```bash
# Check if your commits follow conventional format before pushing
kilo conventional-commit check

# Get a suggested commit message based on staged files
kilo conventional-commit suggest

# Amend the last commit with a validated message
kilo conventional-commit fix --amend
```
