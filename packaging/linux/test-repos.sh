#!/usr/bin/env bash
# Build the apt/dnf/pacman repositories with an EPHEMERAL signing key and verify
# that Debian, Fedora, and Arch containers can add the repo and install bb from
# it. Validates packaging/linux/build-repos.sh end-to-end without the production
# key. Usable locally and in CI (needs docker, gpg, python3, gh).
#
#   TAG=v0.19.1 ./packaging/linux/test-repos.sh   # test against a release's packages
#   DIST_DIR=./dist ./packaging/linux/test-repos.sh  # test against local packages
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
  rm -rf "$WORK"
}
trap cleanup EXIT

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

# Obtain packages: explicit DIST_DIR, else download a release's assets.
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

echo
echo "================ Debian (apt) ================"
docker run --rm --network host debian:stable bash -c "
  set -e
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq && apt-get install -y -qq curl ca-certificates >/dev/null
  curl -fsSL $BASE_URL/bitbucket-cli.gpg | tee /usr/share/keyrings/bitbucket-cli.gpg >/dev/null
  echo 'deb [signed-by=/usr/share/keyrings/bitbucket-cli.gpg] $BASE_URL/deb ./' > /etc/apt/sources.list.d/bitbucket-cli.list
  apt-get update -qq
  apt-get install -y -qq bitbucket-cli
  bb --version
"
echo "Debian: OK"

echo
echo "================ Fedora (dnf) ================"
docker run --rm --network host fedora:latest bash -c "
  set -e
  curl -fsSL -o /etc/yum.repos.d/bitbucket-cli.repo $BASE_URL/rpm/bitbucket-cli.repo
  dnf install -y bitbucket-cli
  bb --version
"
echo "Fedora: OK"

echo
echo "================ Arch (pacman) ================"
docker run --rm --network host archlinux:latest bash -c "
  set -e
  pacman-key --init
  curl -fsSL $BASE_URL/bitbucket-cli.gpg | pacman-key --add -
  pacman-key --lsign-key $FPR
  printf '\n[bitbucket-cli]\nSigLevel = Required\nServer = $BASE_URL/arch/\$arch\n' >> /etc/pacman.conf
  pacman -Sy --noconfirm bitbucket-cli
  bb --version
"
echo "Arch: OK"

echo
echo "==> ALL REPO INSTALL TESTS PASSED"
