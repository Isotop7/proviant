#!/bin/bash
# Export SonarQube/SonarCloud issues to a JSON file for agent consumption.
# Uses SONAR_TOKEN env var for authentication.
#
# Usage:
#   ./scripts/sonarqube-issues-export.sh                           # defaults
#   ./scripts/sonarqube-issues-export.sh -o issues.json            # custom output path
#   ./scripts/sonarqube-issues-export.sh --statuses OPEN           # only open issues
#   ./scripts/sonarqube-issues-export.sh --types BUG,VULNERABILITY # filter by type
#   ./scripts/sonarqube-issues-export.sh --severities BLOCKER,CRITICAL
#
# Env vars:
#   SONAR_TOKEN     (required) authentication token
#   SONAR_URL        base SonarQube URL (default: https://sonarcloud.io)
#   PROJECT_KEY      project key (auto-detected from sonar-project.properties)

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

SONAR_URL="${SONAR_URL:-https://sonarcloud.io}"
API_BASE="${SONAR_URL}/api/issues/search"
OUTPUT_FILE="sonar-issues.json"
STATUSES="OPEN"
EXTRA_PARAMS=""

usage() {
	cat <<'EOF'
Usage: sonarqube-issues-export.sh [options]

Options:
  -o, --output <file>       Output file path (default: sonarqube-issues.json)
  --statuses <list>         Comma-separated: OPEN,CONFIRMED,REOPENED,RESOLVED,CLOSED
  --resolutions <list>      Comma-separated: FALSE-POSITIVE,WONTFIX,FIXED,REMOVED
  --severities <list>       Comma-separated: BLOCKER,CRITICAL,MAJOR,MINOR,INFO
  --types <list>            Comma-separated: BUG,VULNERABILITY,CODE_SMELL
  --tags <list>             Comma-separated tag filter
  -h, --help                Show this help

Env vars:
  SONAR_TOKEN    (required) SonarQube authentication token
  SONAR_URL      Server URL (default: https://sonarcloud.io)
  PROJECT_KEY    Project key (auto-detected from sonar-project.properties)
EOF
	exit 0
}

while [[ $# -gt 0 ]]; do
	case "$1" in
		-o|--output) OUTPUT_FILE="$2"; shift 2 ;;
		--statuses) STATUSES="$2"; shift 2 ;;
		--resolutions|--severities|--types|--tags)
			EXTRA_PARAMS="${EXTRA_PARAMS}&${1#--}=$2"; shift 2 ;;
		-h|--help) usage ;;
		*) echo "Unknown option: $1"; usage ;;
	esac
done

if [ -z "${SONAR_TOKEN:-}" ]; then
	echo "ERROR: SONAR_TOKEN env var is not set."
	exit 1
fi

if [ -z "${PROJECT_KEY:-}" ]; then
	if [ -f "sonar-project.properties" ]; then
		PROJECT_KEY=$(grep -E '^sonar\.projectKey=' sonar-project.properties | cut -d= -f2)
	fi
	if [ -z "${PROJECT_KEY:-}" ]; then
		echo "ERROR: PROJECT_KEY not set and not found in sonar-project.properties"
		exit 1
	fi
fi

echo "Instance: ${SONAR_URL}"
echo "Project:  ${PROJECT_KEY}"

BASE_URL="${API_BASE}?componentKeys=${PROJECT_KEY}&statuses=${STATUSES}${EXTRA_PARAMS}"

total=$(curl -sS -u "${SONAR_TOKEN}:" "${BASE_URL}&ps=1" | jq -r '.total // 0')
echo "Total issues: ${total}"

if [ "$total" -eq 0 ]; then
	echo "[]" > "$OUTPUT_FILE"
	echo "No issues found. Empty output written to ${OUTPUT_FILE}"
	exit 0
fi

page=1
pagesize=500
echo -n "[" > "$OUTPUT_FILE"
first=true

while true; do
	echo -n "  Fetching page ${page} ... "

	resp=$(curl -sS -u "${SONAR_TOKEN}:" "${BASE_URL}&ps=${pagesize}&p=${page}")

	fetched=$(echo "$resp" | jq '.issues | length')
	echo "${fetched} issues"

	if [ "$fetched" -eq 0 ]; then
		break
	fi

	# Strip outer [] from .issues array so comma-separated values remain
	issues_flat=$(echo "$resp" | jq -c '.issues' | sed 's/^\[//; s/\]$//')

	if [ "$first" = true ]; then
		first=false
	else
		echo -n "," >> "$OUTPUT_FILE"
	fi

	echo -n "$issues_flat" >> "$OUTPUT_FILE"

	if [ "$fetched" -lt "$pagesize" ]; then
		break
	fi
	page=$((page + 1))
done

echo "]" >> "$OUTPUT_FILE"

final_count=$(jq 'length' "$OUTPUT_FILE")
echo "Exported ${final_count} issues to ${OUTPUT_FILE}"
