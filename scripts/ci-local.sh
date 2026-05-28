#!/bin/sh
set -eu

usage() {
  printf 'usage: %s clean\n' "$0" >&2
}

if [ "$#" -ne 1 ] || [ "$1" != "clean" ]; then
  usage
  exit 2
fi

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
work_dir=$(mktemp -d)
cleanup() {
  rm -rf "$work_dir"
}
trap cleanup EXIT INT TERM

git -C "$repo_root" archive HEAD | tar -x -C "$work_dir"

(
  cd "$work_dir"
  scripts/secret-scan.sh
  files=$(gofmt -l $(find . -name '*.go' -not -path './.git/*'))
  if [ -n "$files" ]; then
    printf '%s\n' "$files"
    exit 1
  fi
  go vet ./...
  go test ./...
  package_dist=$(mktemp -d)
  DIST_DIR=$package_dist scripts/package-release.sh v0.0.0-local >/dev/null
)

printf 'local clean CI passed\n'
