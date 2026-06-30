#!/usr/bin/env bash
# Build the apt/dnf/pacman repositories with an EPHEMERAL signing key and verify
# that distro containers can add the repo and install bb from it. Validates
# packaging/linux/build-repos.sh end-to-end without the production key.
#
# Default (no env): tests debian:stable, fedora:latest, archlinux:latest.
# Single-image (CI matrix): set TEST_FAMILY=apt|dnf|pacman and TEST_IMAGE=<image>.
#
#   TAG=v0.19.2 ./packaging/linux/test-repos.sh
#   DIST_DIR=./dist ./packaging/linux/test-repos.sh
#   TEST_FAMILY=apt TEST_IMAGE=ubuntu:24.04 ./packaging/linux/test-repos.sh
#
# Needs: docker, gpg, python3, gh.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PORT="${PORT:-8087}"
BASE_URL="http://localhost:${PORT}"
WORK="$(mktemp -d)"
export GNUPGHOME="$WORK/gnupg"
mkdir -p "$GNUPGHOME"
chmod 700 "$GNUPGHOME"

cleanup() {
  [ -n "${SERVER_PID:-}" ] && kill "$SERVER_PID" 2>/dev/null || true
  docker run --rm -v "$WORK":"$WORK" alpine rm -rf "$WORK" 2>/dev/null || rm -rf "$WORK" 2>/dev/null || true
}
trap cleanup EXIT

# --- install tests, one per package family ---

test_apt() {
  local image="$1"
  echo "===== apt: $image ====="
  docker run --rm --network host "$image" bash -c "
    set -e; export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq && apt-get install -y -qq curl ca-certificates >/dev/null
    curl -fsSL $BASE_URL/bitbucket-cli.gpg | tee /usr/share/keyrings/bitbucket-cli.gpg >/dev/null
    echo 'deb [signed-by=/usr/share/keyrings/bitbucket-cli.gpg] $BASE_URL/deb ./' > /etc/apt/sources.list.d/bitbucket-cli.list
    apt-get update -qq && apt-get install -y -qq bitbucket-cli
    bb --version
  "
}

test_dnf() {
  local image="$1"
  echo "===== dnf: $image ====="
  docker run --rm --network host "$image" bash -c "
    set -e
    command -v curl >/dev/null || dnf install -y -q --allowerasing curl >/dev/null
    curl -fsSL -o /etc/yum.repos.d/bitbucket-cli.repo $BASE_URL/rpm/bitbucket-cli.repo
    dnf install -y bitbucket-cli
    bb --version
  "
}

test_pacman() {
  local image="$1"
  echo "===== pacman: $image ====="
  docker run --rm --network host "$image" bash -c "
    set -e
    pacman-key --init >/dev/null 2>&1
    curl -fsSL $BASE_URL/bitbucket-cli.gpg | pacman-key --add - >/dev/null 2>&1
    pacman-key --lsign-key $FPR >/dev/null 2>&1
    printf '\n[bitbucket-cli]\nSigLevel = Required\nServer = $BASE_URL/arch/\$arch\n' >> /etc/pacman.conf
    pacman -Sy --noconfirm bitbucket-cli >/dev/null
    bb --version
  "
}

# --- ephemeral signing key ---
echo "==> Generating ephemeral test signing key"
cat > "$WORK/params" <<'EOF'
%no-protection
Key-Type: RSA
Key-Length: 3072
Key-Usage: sign
Name-Real: bb repo test key
Name-Email: test@example.invalid
Expire-Date: 0
%commit
EOF
gpg --batch --gen-key "$WORK/params" 2>/dev/null
FPR="$(gpg --list-secret-keys --with-colons | awk -F: '/^fpr:/{print $10; exit}')"
echo "    test key: $FPR"

# --- packages: explicit DIST_DIR, else download a release's assets ---
if [ -z "${DIST_DIR:-}" ]; then
  TAG="${TAG:-$(gh release view --repo delabrcd/bitbucket-cli --json tagName -q .tagName)}"
  echo "==> Downloading release $TAG packages"
  DIST_DIR="$WORK/dist"
  mkdir -p "$DIST_DIR"
  gh release download "$TAG" --repo delabrcd/bitbucket-cli \
    -p '*.deb' -p '*.rpm' -p '*.pkg.tar.zst' -D "$DIST_DIR"
fi

echo "==> Building repositories"
OUT="$WORK/public"
DIST_DIR="$DIST_DIR" OUT="$OUT" BASE_URL="$BASE_URL" GPG_FINGERPRINT="$FPR" \
  bash "$REPO_ROOT/packaging/linux/build-repos.sh"

echo "==> Serving $OUT on $BASE_URL"
python3 -m http.server "$PORT" --directory "$OUT" >/dev/null 2>&1 &
SERVER_PID=$!
sleep 1

if [ -n "${TEST_FAMILY:-}" ] && [ -n "${TEST_IMAGE:-}" ]; then
  case "$TEST_FAMILY" in
    apt)    test_apt "$TEST_IMAGE" ;;
    dnf)    test_dnf "$TEST_IMAGE" ;;
    pacman) test_pacman "$TEST_IMAGE" ;;
    *) echo "unknown TEST_FAMILY: $TEST_FAMILY" >&2; exit 2 ;;
  esac
else
  test_apt "debian:stable"
  test_dnf "fedora:latest"
  test_pacman "archlinux:latest"
fi

echo
echo "==> REPO INSTALL TESTS PASSED"
