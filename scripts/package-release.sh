#!/bin/sh
set -eu

usage() {
  printf 'usage: %s vX.Y.Z\n' "$0" >&2
}

if [ "$#" -ne 1 ]; then
  usage
  exit 2
fi

version=$1
case "$version" in
  v*) ;;
  *)
    printf 'version must start with v: %s\n' "$version" >&2
    exit 2
    ;;
esac

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/.." && pwd)

goos=${GOOS:-$(go env GOOS)}
goarch=${GOARCH:-$(go env GOARCH)}
dist_dir=${DIST_DIR:-"$repo_root/dist"}
package_name="agent-canon_${version}_${goos}_${goarch}"
package_dir="$dist_dir/$package_name"
archive_path="$dist_dir/$package_name.tar.gz"
binary_name=agent-canon
if [ "$goos" = "windows" ]; then
  binary_name=agent-canon.exe
fi

rm -rf "$package_dir" "$archive_path"
mkdir -p "$package_dir"

(
  cd "$repo_root"
  GOOS=$goos GOARCH=$goarch go build -trimpath -ldflags="-s -w" -o "$package_dir/$binary_name" ./cmd/agent-canon
)

cp "$repo_root/LICENSE" "$package_dir/LICENSE"
cp "$repo_root/README.md" "$package_dir/README.md"
cp "$repo_root/README.zh-CN.md" "$package_dir/README.zh-CN.md"
cp "$repo_root/README.en.md" "$package_dir/README.en.md"

(
  cd "$dist_dir"
  COPYFILE_DISABLE=1 tar -czf "$archive_path" "$package_name"
)

rm -rf "$package_dir"
printf '%s\n' "$archive_path"
