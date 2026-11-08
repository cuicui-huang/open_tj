#!/bin/bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export ARCHS="${ARCHS:-arm64 x86_64}"
bash scripts/build.sh

VERSION="$(tr -d '\n' < VERSION)"
LABEL="$(echo "$ARCHS" | tr ' ' '-')"
case "$ARCHS" in
  "arm64 x86_64"|"x86_64 arm64") TARGET=darwin-universal ;;
  arm64) TARGET=darwin-arm64 ;;
  x86_64) TARGET=darwin-amd64 ;;
  *) echo "Unsupported ARCHS: $ARCHS" >&2; exit 1 ;;
esac
APP="$ROOT/build/$TARGET/Open TJ.app"
[ -d "$APP" ] || { echo "Missing $APP" >&2; exit 1; }

mkdir -p dist
ZIP="dist/OpenTJ-$VERSION-$LABEL.zip"
rm -f "$ZIP"
ditto -c -k --sequesterRsrc --keepParent "$APP" "$ZIP"
shasum -a 256 "$ZIP" > "$ZIP.sha256"
printf 'Packaged: %s\n' "$ZIP"
