#!/bin/bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
sips -z 1024 1024 "$ROOT/assets/logo.png" --out "$ROOT/apps/desktop/resources/icon.png" >/dev/null
printf 'Wrote %s\n' "$ROOT/apps/desktop/resources/icon.png"
