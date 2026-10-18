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
#   PROJECT_KEY      project key (default: Isotop7_proviant2)

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

SONAR_URL="${SONAR_URL:-https://sonarcloud.io}"
API_BASE="${SONAR_URL}/api/issues/search"
OUTPUT_FILE="sonar-export.json"
STATUSES="OPEN"
EXTRA_PARAMS=""
MODE="issues"

usage() {
	cat <<'EOF'
Usage: sonarqube-issues-export.sh [options]

Options:
  -o, --output <file>       Output file path (default: sonar-issues.json)
  --statuses <list>         Comma-separated: OPEN,CONFIRMED,REOPENED,RESOLVED,CLOSED
  --resolutions <list>      Comma-separated: FALSE-POSITIVE,WONTFIX,FIXED,REMOVED
  --severities <list>       Comma-separated: BLOCKER,CRITICAL,MAJOR,MINOR,INFO
  --types <list>            Comma-separated: BUG,VULNERABILITY,CODE_SMELL
  --tags <list>             Comma-separated tag filter
  --duplications            Export code duplication metrics instead of issues
  -h, --help                Show this help

Env vars:
  SONAR_TOKEN    (required) SonarQube authentication token
  SONAR_URL      Server URL (default: https://sonarcloud.io)
  PROJECT_KEY    Project key (default: Isotop7_proviant2)
EOF
	exit 0
}

while [[ $# -gt 0 ]]; do
	case "$1" in
		-o|--output) OUTPUT_FILE="$2"; shift 2 ;;
		--statuses) STATUSES="$2"; shift 2 ;;
		--resolutions|--severities|--types|--tags)
			EXTRA_PARAMS="${EXTRA_PARAMS}&${1#--}=$2"; shift 2 ;;
		--duplications) MODE="duplications"; shift ;;
		-h|--help) usage ;;
		*) echo "Unknown option: $1"; usage ;;
	esac
done

if [[ -z "${SONAR_TOKEN:-}" ]]; then
	echo "ERROR: SONAR_TOKEN env var is not set."
	exit 1
fi

PROJECT_KEY="${PROJECT_KEY:-Isotop7_proviant2}"

# A rejected token (401) answers with an empty body, which every JSON read below would
# silently turn into "no findings" — an auth failure must never look like a clean report.
sonar_get() {
	local response
	if ! response=$(curl -sS -u "${SONAR_TOKEN}:" "$1") || [[ -z "$response" ]]; then
		echo "ERROR: empty response from ${1%%\?*}" >&2
		exit 1
	fi
	if echo "$response" | jq -e '.errors' >/dev/null 2>&1; then
		echo "ERROR: $(echo "$response" | jq -r '.errors[].msg')" >&2
		exit 1
	fi
	printf '%s' "$response"
}

echo "Instance: ${SONAR_URL}"
echo "Project:  ${PROJECT_KEY}"

if [[ "$MODE" = "duplications" ]]; then
	DUPLICATION_METRICS="duplicated_lines_density,duplicated_lines,duplicated_blocks"
	TREE_BASE="${SONAR_URL}/api/measures/component_tree?component=${PROJECT_KEY}&metricKeys=${DUPLICATION_METRICS}&qualifiers=FIL&strategy=leaves&ps=500"

	page=1
	all_files="[]"

	while true; do
		echo -n "  Fetching page ${page} ..."
		resp=$(sonar_get "${TREE_BASE}&p=${page}")

		fetched=$(echo "$resp" | jq '.components | length')
		total_pages=$(echo "$resp" | jq '.paging | ceil(.total / .pageSize)' 2>/dev/null || echo "$page")
		echo " ${fetched} files"

		batch=$(echo "$resp" | jq '[.components[] | {
			path: .path,
			name: .name,
			duplicated_lines_density: (.measures[] | select(.metric == "duplicated_lines_density") | .value | tonumber),
			duplicated_lines: (.measures[] | select(.metric == "duplicated_lines") | .value | tonumber),
			duplicated_blocks: (.measures[] | select(.metric == "duplicated_blocks") | .value | tonumber)
		}]')

		all_files=$(echo "$all_files $batch" | jq -s '.[0] + .[1]')

		if [[ "$fetched" -lt 500 ]]; then
			break
		fi
		page=$((page + 1))
	done

	echo "$all_files" | jq 'sort_by(-.duplicated_lines_density)' > "$OUTPUT_FILE"

	final_count=$(jq 'length' "$OUTPUT_FILE")
	echo "Exported ${final_count} files to ${OUTPUT_FILE}"
	exit 0
fi

BASE_URL="${API_BASE}?componentKeys=${PROJECT_KEY}&statuses=${STATUSES}${EXTRA_PARAMS}"

total=$(sonar_get "${BASE_URL}&ps=1" | jq -r '.total // 0')
echo "Total issues: ${total}"

if [[ "$total" -eq 0 ]]; then
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

	resp=$(sonar_get "${BASE_URL}&ps=${pagesize}&p=${page}")

	fetched=$(echo "$resp" | jq '.issues | length')
	echo "${fetched} issues"

	if [[ "$fetched" -eq 0 ]]; then
		break
	fi

	# Strip outer [] from .issues array so comma-separated values remain
	issues_flat=$(echo "$resp" | jq -c '.issues' | sed 's/^\[//; s/\]$//')

	if [[ "$first" = true ]]; then
		first=false
	else
		echo -n "," >> "$OUTPUT_FILE"
	fi

	echo -n "$issues_flat" >> "$OUTPUT_FILE"

	if [[ "$fetched" -lt "$pagesize" ]]; then
		break
	fi
	page=$((page + 1))
done

echo "]" >> "$OUTPUT_FILE"

final_count=$(jq 'length' "$OUTPUT_FILE")
echo "Exported ${final_count} issues to ${OUTPUT_FILE}"
