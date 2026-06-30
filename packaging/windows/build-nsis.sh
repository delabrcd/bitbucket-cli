#!/usr/bin/env bash
# Build the Windows NSIS installer for bb from the GoReleaser-built windows/amd64
# binary, using makensis. Requires makensis and the EnVar plugin installed.
#
# Usage: build-nsis.sh <version>
#   DIST_DIR may override the GoReleaser dist directory (default: ./dist).
set -euo pipefail

VERSION="${1:?usage: build-nsis.sh <version>}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NSI="$SCRIPT_DIR/bb.nsi"
DIST_DIR="${DIST_DIR:-dist}"
DIST_ABS="$(cd "$DIST_DIR" && pwd)"

if ! command -v makensis >/dev/null 2>&1; then
  echo "build-nsis.sh: makensis not found on PATH; skipping installer build" >&2
  exit 0
fi

shopt -s nullglob
built=0
for exedir in "$DIST_ABS"/bb_windows_amd64*/; do
  exe="${exedir}bb.exe"
  [ -f "$exe" ] || continue
  out="$DIST_ABS/bitbucket-cli_${VERSION}_windows_amd64-setup.exe"
  tmp="$(mktemp -d)"
  cp "$exe" "$tmp/bb.exe"
  makensis -V2 -DVERSION="$VERSION" -DOUTFILE="$out" -DSRCDIR="$tmp" "$NSI"
  rm -rf "$tmp"
  echo "Built installer: $out"
  built=1
done

if [ "$built" -eq 0 ]; then
  echo "build-nsis.sh: no windows/amd64 build found under $DIST_ABS; skipping installer" >&2
fi
