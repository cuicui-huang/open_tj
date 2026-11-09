#!/bin/bash
# Build every supported target. Outputs go to build/<goos>-<goarch>/.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/apps/desktop"

targets=(
  darwin/arm64
  darwin/amd64
  windows/arm64
  windows/amd64
)
for target in "${targets[@]}"; do
  printf '\n=== %s ===\n' "$target"
  case "$target" in
    windows/*)
      go tool mygo build -platform "$target" -skip-installer
      ;;
    *)
      go tool mygo build -platform "$target"
      ;;
  esac
done
