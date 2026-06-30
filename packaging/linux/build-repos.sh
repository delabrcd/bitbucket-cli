#!/usr/bin/env bash
set -euo pipefail

# Required environment variables
DIST_DIR="${DIST_DIR:-dist}"
OUT="${OUT:-public}"
BASE_URL="${BASE_URL:?BASE_URL is required}"
GPG_FINGERPRINT="${GPG_FINGERPRINT:?GPG_FINGERPRINT is required}"

echo "==> Building Linux package repositories"
echo "    DIST_DIR:        $DIST_DIR"
echo "    OUT:             $OUT"
echo "    BASE_URL:        $BASE_URL"
echo "    GPG_FINGERPRINT: $GPG_FINGERPRINT"

# Step 1: clean output directory
rm -rf "$OUT"
mkdir -p "$OUT"

# Step 2: export public key in both armored and binary form
echo "==> Exporting public signing key"
gpg --armor --export "$GPG_FINGERPRINT" > "$OUT/bitbucket-cli.asc"
gpg --export "$GPG_FINGERPRINT" > "$OUT/bitbucket-cli.gpg"

# Step 3: APT flat repository
echo "==> Building APT repository"
mkdir -p "$OUT/deb"
cp $DIST_DIR/*.deb "$OUT/deb/" 2>/dev/null || true
if compgen -G "$OUT/deb/*.deb" > /dev/null 2>&1; then
    pushd "$OUT/deb" > /dev/null
    apt-ftparchive packages . > Packages
    gzip -kf Packages
    apt-ftparchive release . > Release
    gpg --batch --yes --default-key "$GPG_FINGERPRINT" \
        --clearsign -o InRelease Release
    gpg --batch --yes --default-key "$GPG_FINGERPRINT" \
        -abs -o Release.gpg Release
    popd > /dev/null
    echo "    APT repository built."
else
    echo "    No .deb packages found, skipping APT repository."
fi

# Step 4: RPM repository
echo "==> Building RPM repository"
mkdir -p "$OUT/rpm"
cp $DIST_DIR/*.rpm "$OUT/rpm/" 2>/dev/null || true
if compgen -G "$OUT/rpm/*.rpm" > /dev/null 2>&1; then
    # Sign each RPM package so dnf's gpgcheck=1 passes (nfpm ships them unsigned).
    # Done before createrepo_c so the metadata checksums match the signed files.
    printf '%%_gpg_name %s\n' "$GPG_FINGERPRINT" > "$HOME/.rpmmacros"
    rpm --addsign "$OUT"/rpm/*.rpm
    pushd "$OUT/rpm" > /dev/null
    createrepo_c .
    gpg --batch --yes --default-key "$GPG_FINGERPRINT" \
        --detach-sign --armor repodata/repomd.xml
    popd > /dev/null

    # Write .repo file for dnf/yum
    cat > "$OUT/rpm/bitbucket-cli.repo" <<EOF
[bitbucket-cli]
name=Bitbucket CLI
baseurl=$BASE_URL/rpm
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=$BASE_URL/bitbucket-cli.asc
EOF
    echo "    RPM repository built."
else
    echo "    No .rpm packages found, skipping RPM repository."
fi

# Step 5: Pacman (Arch) repository — one sub-repo per architecture.
# Both arch packages share pkgname-pkgver (bitbucket-cli-0.19.1-1), so they
# cannot live in one db; pacman uses Server = .../arch/$arch to pick x86_64 vs
# aarch64.
echo "==> Building Pacman repository"
if compgen -G "$DIST_DIR/*.pkg.tar.zst" > /dev/null 2>&1; then
    for pkg in "$DIST_DIR"/*.pkg.tar.zst; do
        case "$(basename "$pkg")" in
            *amd64*) parch=x86_64 ;;
            *arm64*) parch=aarch64 ;;
            *)       parch=any ;;
        esac
        mkdir -p "$OUT/arch/$parch"
        cp "$pkg" "$OUT/arch/$parch/"
    done

    for archdir in "$OUT/arch"/*/; do
        [ -d "$archdir" ] || continue
        echo "    Building $(basename "$archdir") sub-repo"
        # Generate the DB inside an Arch container (no signing in Docker).
        # Use an absolute path for the bind mount so it works whether OUT is
        # relative or absolute (the host daemon resolves the mount path).
        archabs="$(cd "$archdir" && pwd)"
        docker run --rm -v "$archabs":/repo -w /repo archlinux:latest \
            bash -c "repo-add bitbucket-cli.db.tar.zst *.pkg.tar.zst"
        # Sign each package and the DB on the runner (outside Docker). pacman
        # fetches "<repo>.db" + "<repo>.db.sig", so the DB sig is named .db.sig.
        for pkg in "$archdir"*.pkg.tar.zst; do
            gpg --batch --yes --default-key "$GPG_FINGERPRINT" \
                --detach-sign --no-armor "$pkg"
        done
        gpg --batch --yes --default-key "$GPG_FINGERPRINT" \
            --detach-sign --no-armor -o "${archdir}bitbucket-cli.db.sig" "${archdir}bitbucket-cli.db.tar.zst"
    done
    echo "    Pacman repository built (per-architecture)."
else
    echo "    No .pkg.tar.zst packages found, skipping Pacman repository."
fi

# Step 6: index.html with install instructions
echo "==> Writing index.html"
cat > "$OUT/index.html" <<'HTMLEOF'
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>bb — Bitbucket CLI Linux Package Repositories</title>
<style>
  body { font-family: sans-serif; max-width: 800px; margin: 2rem auto; padding: 0 1rem; line-height: 1.6; }
  h1 { border-bottom: 1px solid #ccc; padding-bottom: .5rem; }
  h2 { margin-top: 2rem; }
  pre { background: #f4f4f4; border: 1px solid #ddd; border-radius: 4px; padding: 1rem; overflow-x: auto; }
  code { font-family: monospace; }
  .note { background: #fffbe6; border-left: 4px solid #e6b800; padding: .75rem 1rem; margin: 1rem 0; }
</style>
</head>
<body>
<h1>bb &mdash; Bitbucket CLI Linux Package Repositories</h1>
<p>
  Add the appropriate repository below to keep <code>bb</code> up to date with your package manager.
  Normal <code>apt upgrade</code>, <code>dnf upgrade</code>, and <code>pacman -Syu</code> will
  automatically pick up new releases once the repo is configured.
</p>

<h2>Debian / Ubuntu (apt)</h2>
<pre><code>curl -fsSL https://delabrcd.github.io/bitbucket-cli/bitbucket-cli.gpg | sudo tee /usr/share/keyrings/bitbucket-cli.gpg &gt; /dev/null
echo "deb [signed-by=/usr/share/keyrings/bitbucket-cli.gpg] https://delabrcd.github.io/bitbucket-cli/deb ./" | sudo tee /etc/apt/sources.list.d/bitbucket-cli.list
sudo apt update &amp;&amp; sudo apt install bitbucket-cli</code></pre>

<h2>Fedora / RHEL (dnf)</h2>
<pre><code>sudo curl -fsSL -o /etc/yum.repos.d/bitbucket-cli.repo https://delabrcd.github.io/bitbucket-cli/rpm/bitbucket-cli.repo
sudo dnf install bitbucket-cli</code></pre>

<h2>Arch Linux (pacman)</h2>
<pre><code>curl -fsSL https://delabrcd.github.io/bitbucket-cli/bitbucket-cli.gpg | sudo pacman-key --add -
sudo pacman-key --lsign-key 4AEA6A46957261ECE8F77071AE688AB6D9A1A1CA

# Add to /etc/pacman.conf:
[bitbucket-cli]
Server = https://delabrcd.github.io/bitbucket-cli/arch/$arch

# Then install:
sudo pacman -Sy bitbucket-cli</code></pre>

<div class="note">
  Packages are GPG-signed. Key fingerprint: <code>4AEA6A46957261ECE8F77071AE688AB6D9A1A1CA</code>
</div>
</body>
</html>
HTMLEOF

echo "==> Done. Output written to: $OUT"
