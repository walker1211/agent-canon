#!/bin/sh
set -eu

root=
history=false

usage() {
  printf 'usage: %s [--root <path>] [--history]\n' "$0" >&2
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --root)
      if [ "$#" -lt 2 ]; then
        usage
        exit 2
      fi
      root=$2
      shift 2
      ;;
    --history)
      history=true
      shift
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
default_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
root=${root:-$default_root}

if ! git -C "$root" rev-parse --git-dir >/dev/null 2>&1; then
  printf 'not a git repository: %s\n' "$root" >&2
  exit 2
fi

found=0

report_finding() {
  path=$1
  reason=$2
  printf 'secret scan finding: %s: %s <REDACTED>\n' "$path" "$reason" >&2
  found=1
}

is_ignored_fixture() {
  case "$1" in
    testdata/secrets/*|*_test.go) return 0 ;;
    *) return 1 ;;
  esac
}

scan_file() {
  rel=$1
  path=$root/$rel
  if is_ignored_fixture "$rel"; then
    return
  fi
  case "$rel" in
    .env|.env.*|configs/config.yaml|*.pem|*.key|*.crt|*id_rsa*|*id_ed25519*)
      report_finding "$rel" "sensitive filename"
      ;;
  esac
  if [ ! -f "$path" ]; then
    return
  fi
  if grep -Eq 'gh[pousr]_[A-Za-z0-9]{36,}|(GITHUB_TOKEN|ANTHROPIC_API_KEY|OPENAI_API_KEY|API_KEY|TOKEN|SECRET|PASSWORD)[[:space:]]*=[[:space:]]*[A-Za-z0-9_./+-]{8,}|-----BEGIN (RSA |OPENSSH |EC |DSA )?PRIVATE KEY-----' "$path" 2>/dev/null; then
    report_finding "$rel" "secret-like content"
  fi
}

if [ "$history" = true ]; then
  shallow=$(git -C "$root" rev-parse --is-shallow-repository 2>/dev/null || printf 'false')
  if [ "$shallow" = true ]; then
    printf 'history secret scan requires a full clone; repository is shallow\n' >&2
    exit 2
  fi
  git -C "$root" rev-list --objects --all | while IFS=' ' read -r object rel; do
    if [ -z "${rel:-}" ]; then
      continue
    fi
    if is_ignored_fixture "$rel"; then
      continue
    fi
    if git -C "$root" cat-file -p "$object" 2>/dev/null | grep -Eq 'gh[pousr]_[A-Za-z0-9]{36,}|(GITHUB_TOKEN|ANTHROPIC_API_KEY|OPENAI_API_KEY|API_KEY|TOKEN|SECRET|PASSWORD)[[:space:]]*=[[:space:]]*[A-Za-z0-9_./+-]{8,}|-----BEGIN (RSA |OPENSSH |EC |DSA )?PRIVATE KEY-----'; then
      printf 'secret scan finding: %s: historical secret-like content <REDACTED>\n' "$rel" >&2
      exit 1
    fi
  done
fi

for rel in $(git -C "$root" ls-files); do
  scan_file "$rel"
done

if [ "$found" -ne 0 ]; then
  exit 1
fi

printf 'secret scan passed\n'
