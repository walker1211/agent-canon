#!/bin/sh
set -eu

usage() {
  printf 'usage: %s vX.Y.Z\n' "$0" >&2
}

if [ "$#" -ne 1 ]; then
  usage
  exit 2
fi

tag=$1
case "$tag" in
  v*) ;;
  *)
    printf 'release tag must start with v: %s\n' "$tag" >&2
    exit 2
    ;;
esac

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/.." && pwd)

if [ -n "$(git -C "$repo_root" status --porcelain)" ]; then
  printf 'working tree is not clean; commit or stash changes before tagging\n' >&2
  exit 1
fi

(
  cd "$repo_root"
  scripts/ci-local.sh clean
  git tag "$tag"
)

printf 'created local release tag %s\n' "$tag"
printf 'review GitHub release readiness, then push explicitly with: git push origin %s\n' "$tag"
