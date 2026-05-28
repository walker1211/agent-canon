#!/bin/sh
set -eu

repo=
branch=
mode=report-only

usage() {
  printf 'usage: %s [--repo OWNER/REPO] [--branch BRANCH] [--strict|--report-only]\n' "$0" >&2
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --repo)
      if [ "$#" -lt 2 ]; then
        usage
        exit 2
      fi
      repo=$2
      shift 2
      ;;
    --branch)
      if [ "$#" -lt 2 ]; then
        usage
        exit 2
      fi
      branch=$2
      shift 2
      ;;
    --strict)
      mode=strict
      shift
      ;;
    --report-only)
      mode=report-only
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --*)
      usage
      exit 2
      ;;
    *)
      if [ -n "$repo" ]; then
        usage
        exit 2
      fi
      repo=$1
      shift
      ;;
  esac
done

missing_dependency=0
if ! command -v gh >/dev/null 2>&1; then
  printf 'missing dependency: gh\n' >&2
  missing_dependency=1
fi
if ! command -v jq >/dev/null 2>&1; then
  printf 'missing dependency: jq\n' >&2
  missing_dependency=1
fi
if [ "$missing_dependency" -ne 0 ]; then
  printf 'install missing dependencies and authenticate gh before running readiness checks\n' >&2
  exit 2
fi

if [ -z "$repo" ]; then
  repo=$(gh repo view --json nameWithOwner --jq .nameWithOwner 2>/dev/null || true)
fi

if [ -z "$repo" ]; then
  printf 'repository: unavailable; pass --repo OWNER/REPO or run inside a GitHub repository\n' >&2
  exit 2
fi

printf 'Repository: %s\n' "$repo"
printf 'Mode: %s\n' "$mode"
printf '\n'

missing=0

print_status() {
  label=$1
  value=$2
  printf '%s: %s\n' "$label" "$value"
}

mark_not_ready() {
  label=$1
  value=$2
  guidance=$3
  print_status "$label" "$value"
  printf '  remediation: %s\n' "$guidance"
  missing=1
}

require_enabled_status() {
  label=$1
  value=$2
  guidance=$3
  if [ "$value" = enabled ]; then
    print_status "$label" "$value"
  else
    mark_not_ready "$label" "$value" "$guidance"
  fi
}

repo_json=$(gh api "repos/$repo" 2>/dev/null || true)
if [ -z "$repo_json" ]; then
  mark_not_ready "repository metadata" "unavailable" "check gh authentication and repository access"
else
  secret_scanning=$(printf '%s' "$repo_json" | jq -r '.security_and_analysis.secret_scanning.status // "unavailable"')
  push_protection=$(printf '%s' "$repo_json" | jq -r '.security_and_analysis.secret_scanning_push_protection.status // "unavailable"')
  require_enabled_status "secret scanning" "$secret_scanning" "enable Secret scanning in Settings > Code security and analysis"
  require_enabled_status "secret scanning push protection" "$push_protection" "enable push protection in Settings > Code security and analysis"
  if [ -z "$branch" ]; then
    branch=$(printf '%s' "$repo_json" | jq -r '.default_branch // ""')
  fi
fi

if [ -n "$branch" ]; then
  print_status "default branch" "$branch"
else
  mark_not_ready "default branch" "unavailable" "check repository metadata access or pass --branch explicitly"
fi

pvr_json=$(gh api "repos/$repo/private-vulnerability-reporting" 2>/dev/null || true)
if [ -z "$pvr_json" ]; then
  mark_not_ready "private vulnerability reporting" "unavailable" "enable private vulnerability reporting or check public repository/admin permission"
else
  pvr_enabled=$(printf '%s' "$pvr_json" | jq -r '.enabled // false')
  if [ "$pvr_enabled" = true ]; then
    print_status "private vulnerability reporting" "$pvr_enabled"
  else
    mark_not_ready "private vulnerability reporting" "$pvr_enabled" "enable private vulnerability reporting in repository Security settings"
  fi
fi

branch_ready=false
if [ -n "$branch" ]; then
  branch_json=$(gh api "repos/$repo/branches/$branch/protection" 2>/dev/null || true)
  if [ -n "$branch_json" ]; then
    required_checks=$(printf '%s' "$branch_json" | jq -r 'if .required_status_checks then "enabled" else "missing" end')
    if [ "$required_checks" = enabled ]; then
      print_status "$branch branch protection" "enabled; required status checks enabled"
      branch_ready=true
    else
      mark_not_ready "$branch branch protection" "enabled; required status checks missing" "require CI status checks on the default branch protection rule"
    fi
  fi
fi

if [ "$branch_ready" != true ]; then
  rulesets_json=$(gh api "repos/$repo/rulesets" 2>/dev/null || true)
  if [ -n "$rulesets_json" ] && [ "$(printf '%s' "$rulesets_json" | jq 'length')" -gt 0 ]; then
    print_status "default branch protection or rulesets" "rulesets present"
  else
    mark_not_ready "default branch protection or rulesets" "missing or unavailable" "add a main/default branch protection rule or repository ruleset requiring CI"
  fi
fi

code_scanning_json=$(gh api "repos/$repo/code-scanning/alerts?state=open&per_page=1" 2>/dev/null || true)
if [ -z "$code_scanning_json" ]; then
  mark_not_ready "CodeQL / code-scanning" "unavailable" "enable CodeQL or code scanning and ensure security-events read permission"
else
  open_alerts=$(printf '%s' "$code_scanning_json" | jq 'length')
  print_status "CodeQL / code-scanning" "available; open alerts sample count $open_alerts"
fi

if [ "$mode" = strict ] && [ "$missing" -ne 0 ]; then
  exit 1
fi
