#!/bin/bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/apps/desktop"

ARCHS="${ARCHS:-$(uname -m)}"
case "$ARCHS" in
  "arm64 x86_64"|"x86_64 arm64") PLATFORM=darwin/universal ;;
  arm64) PLATFORM=darwin/arm64 ;;
  x86_64) PLATFORM=darwin/amd64 ;;
  *) echo "Unsupported ARCHS: $ARCHS" >&2; exit 1 ;;
esac

args=(-platform "$PLATFORM")
if [[ -n "${SIGN_IDENTITY:-}" ]]; then
  args+=(-sign "$SIGN_IDENTITY")
fi
go tool mygo build "${args[@]}"
