#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

cd "$script_dir"
go build -o agent-canon ./cmd/agent-canon
