#!/usr/bin/env bash
# Build a Windows MSI for bb from the GoReleaser-built windows/amd64 binary,
# using wixl (msitools). Invoked as a GoReleaser "after" hook and re-run by CI.
#
# Usage: build-msi.sh <version>
#   DIST_DIR may override the GoReleaser dist directory (default: ./dist).
set -euo pipefail

VERSION="${1:?usage: build-msi.sh <version>}"
# MSI ProductVersion must be strictly numeric x.y.z — strip a leading v and any
# pre-release/build suffix (e.g. "0.18.3-next" -> "0.18.3").
MSI_VERSION="$(printf '%s' "$VERSION" | sed -E 's/^v//; s/[^0-9.].*$//')"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WXS="$SCRIPT_DIR/bb.wxs"
DIST_DIR="${DIST_DIR:-dist}"
DIST_ABS="$(cd "$DIST_DIR" && pwd)"

if ! command -v wixl >/dev/null 2>&1; then
  echo "build-msi.sh: wixl (msitools) not found on PATH; skipping MSI build" >&2
  exit 0
fi

shopt -s nullglob
built=0
for exedir in "$DIST_ABS"/bb_windows_amd64*/; do
  exe="${exedir}bb.exe"
  [ -f "$exe" ] || continue
  out="$DIST_ABS/bitbucket-cli_${VERSION}_windows_amd64.msi"
  tmp="$(mktemp -d)"
  cp "$exe" "$tmp/bb.exe"
  ( cd "$tmp" && wixl -D Version="$MSI_VERSION" -o "$out" "$WXS" )
  rm -rf "$tmp"
  echo "Built MSI: $out"
  built=1
done

if [ "$built" -eq 0 ]; then
  echo "build-msi.sh: no windows/amd64 build found under $DIST_ABS; skipping MSI" >&2
fi
