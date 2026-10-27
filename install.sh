#!/bin/sh
# Installs the latest ha-notify release for Linux or macOS.
#   curl -fsSL https://raw.githubusercontent.com/outsharked/ha-notify/master/install.sh | sh
# Environment: INSTALL_DIR (default ~/.local/bin), VERSION (default latest, e.g. v0.1.0).
set -eu

REPO="outsharked/ha-notify"
BIN="ha-notify"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${VERSION:-latest}"

say() { printf '%s\n' "$*" >&2; }
die() { say "install: $*"; exit 1; }

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) die "unsupported OS $(uname -s); build from source with 'go install github.com/$REPO@latest'" ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) die "unsupported CPU $(uname -m)" ;;
esac
asset="$BIN-$os-$arch"

# HA_NOTIFY_BASE_URL overrides the download location (used by the install test).
if [ -n "${HA_NOTIFY_BASE_URL:-}" ]; then
  base="$HA_NOTIFY_BASE_URL"
elif [ "$VERSION" = latest ]; then
  base="https://github.com/$REPO/releases/latest/download"
else
  base="https://github.com/$REPO/releases/download/$VERSION"
fi

fetch() { # url dest
  if command -v curl >/dev/null 2>&1; then curl -fsSL -o "$2" "$1"
  elif command -v wget >/dev/null 2>&1; then wget -qO "$2" "$1"
  else die "need curl or wget"; fi
}
sha256() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | awk '{print $1}'
  else die "need sha256sum or shasum to verify the download"; fi
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

say "Downloading $asset ($VERSION)..."
fetch "$base/$asset" "$tmp/$asset" || die "download failed: $base/$asset"
fetch "$base/checksums.txt" "$tmp/checksums.txt" || die "download failed: $base/checksums.txt"

want=$(awk -v f="$asset" '$2 == f {print $1}' "$tmp/checksums.txt")
[ -n "$want" ] || die "$asset is not listed in checksums.txt"
got=$(sha256 "$tmp/$asset")
[ "$want" = "$got" ] || die "checksum mismatch for $asset (expected $want, got $got)"

mkdir -p "$INSTALL_DIR"
chmod 755 "$tmp/$asset"
mv "$tmp/$asset" "$INSTALL_DIR/$BIN"
say "Installed $("$INSTALL_DIR/$BIN" version) to $INSTALL_DIR/$BIN"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) say "Note: $INSTALL_DIR is not on your PATH; add it to your shell profile." ;;
esac
say "Next: create a config and token, see https://github.com/$REPO#setup"
