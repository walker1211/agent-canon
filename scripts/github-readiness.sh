#!/bin/sh
set -eu

repo=${1:-}
if [ -z "$repo" ]; then
  repo=$(gh repo view --json nameWithOwner --jq .nameWithOwner)
fi

if [ -z "$repo" ]; then
  printf 'repository: unavailable\n'
  exit 1
fi

printf 'Repository: %s\n' "$repo"
printf '\n'

print_status() {
  label=$1
  value=$2
  printf '%s: %s\n' "$label" "$value"
}

repo_json=$(gh api "repos/$repo" 2>/dev/null || true)
if [ -z "$repo_json" ]; then
  print_status "repository metadata" "unavailable; check gh authentication and repository access"
else
  secret_scanning=$(printf '%s' "$repo_json" | jq -r '.security_and_analysis.secret_scanning.status // "unavailable"')
  push_protection=$(printf '%s' "$repo_json" | jq -r '.security_and_analysis.secret_scanning_push_protection.status // "unavailable"')
  print_status "secret scanning" "$secret_scanning"
  print_status "secret scanning push protection" "$push_protection"
fi

pvr_json=$(gh api "repos/$repo/private-vulnerability-reporting" 2>/dev/null || true)
if [ -z "$pvr_json" ]; then
  print_status "private vulnerability reporting" "unavailable; may require public repository, GitHub Advanced Security, or admin permission"
else
  pvr_enabled=$(printf '%s' "$pvr_json" | jq -r '.enabled // false')
  print_status "private vulnerability reporting" "$pvr_enabled"
fi

branch_json=$(gh api "repos/$repo/branches/main/protection" 2>/dev/null || true)
if [ -n "$branch_json" ]; then
  required_checks=$(printf '%s' "$branch_json" | jq -r 'if .required_status_checks then "enabled" else "missing" end')
  print_status "main branch protection" "enabled; required status checks $required_checks"
else
  rulesets_json=$(gh api "repos/$repo/rulesets" 2>/dev/null || true)
  if [ -n "$rulesets_json" ] && [ "$(printf '%s' "$rulesets_json" | jq 'length')" -gt 0 ]; then
    print_status "main branch protection or rulesets" "rulesets present"
  else
    print_status "main branch protection or rulesets" "missing or unavailable"
  fi
fi

code_scanning_json=$(gh api "repos/$repo/code-scanning/alerts?state=open&per_page=1" 2>/dev/null || true)
if [ -z "$code_scanning_json" ]; then
  print_status "CodeQL / code-scanning" "unavailable; enable code scanning or check permission"
else
  open_alerts=$(printf '%s' "$code_scanning_json" | jq 'length')
  print_status "CodeQL / code-scanning" "available; open alerts sample count $open_alerts"
fi
