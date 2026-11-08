#!/bin/bash
# Collect one platform's build into dist/ with unique names and checksums.
# Usage: scripts/ci-collect.sh <goos>/<goarch>
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

PLATFORM="${1:?platform, e.g. darwin/arm64}"
VERSION="$(tr -d '\n' < VERSION)"
OS="${PLATFORM%/*}"
ARCH="${PLATFORM#*/}"
DIR="$OS-$ARCH"
SRC="build/$DIR"
OUT="dist/$DIR"
[ -d "$SRC" ] || { echo "Missing $SRC" >&2; exit 1; }
rm -rf "$OUT"
mkdir -p "$OUT"

case "$OS" in
  darwin)
    APP="$SRC/Open TJ.app"
    [ -d "$APP" ] || { echo "Missing $APP" >&2; exit 1; }
    codesign --verify --deep --strict "$APP"
    test -s "$APP/Contents/Resources/AppIcon.icns"
    test -s "$APP/Contents/Resources/THIRD_PARTY_NOTICES.md"
    ditto -c -k --sequesterRsrc --keepParent "$APP" "$OUT/OpenTJ-$VERSION-$DIR.zip"
    cp "$SRC"/*.dmg "$OUT/OpenTJ-$VERSION-$DIR.dmg"
    ;;
  windows)
    found=0
    for file in "$SRC"/*.exe; do
      [ -e "$file" ] || continue
      found=1
      name="$(basename "$file" .exe | tr ' ' '-')"
      cp "$file" "$OUT/$name-$VERSION-$DIR.exe"
    done
    [ "$found" = 1 ] || { echo "No .exe in $SRC" >&2; exit 1; }
    ;;
  linux)
    shopt -s nullglob
    files=("$SRC"/*.tar.gz "$SRC"/*.deb)
    [ ${#files[@]} -gt 0 ] || { echo "No Linux packages in $SRC" >&2; exit 1; }
    cp "${files[@]}" "$OUT/"
    ;;
  *)
    echo "Unsupported platform: $PLATFORM" >&2
    exit 1
    ;;
esac

if [ -d "$OUT" ]; then
  (cd "$OUT" && shasum -a 256 * > "SHA256SUMS-$DIR")
fi
printf 'Collected %s\n' "$PLATFORM"
ls -l "$OUT"
