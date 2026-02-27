#!/usr/bin/env bash
set -euo pipefail

# LazyArgo installer
# - Downloads a tagged release asset for your OS/arch
# - Verifies SHA256 using checksums.txt
# - Installs binary to a target directory

usage() {
  cat <<'USAGE'
Usage:
  install.sh [VERSION] [-b DIR]

Examples:
  ./install.sh v0.1.0-alpha.2
  curl -fsSL https://raw.githubusercontent.com/phin3has/lazyargo/main/scripts/install.sh | bash -s -- v0.1.0-alpha.2 -b /usr/local/bin

Options:
  -b DIR   Install directory (default: ./bin)

Notes:
  - Requires: curl, tar, sha256sum
  - macOS: you may need coreutils for sha256sum (or set SHASUM=shasum)
USAGE
}

BIN_DIR="$(pwd)/bin"
VERSION=""

# Allow VERSION as first arg.
if [[ ${1:-} == v* ]]; then
  VERSION="$1"
  shift
fi

while getopts ":b:h" opt; do
  case "$opt" in
    b) BIN_DIR="$OPTARG" ;;
    h) usage; exit 0 ;;
    *) usage; exit 1 ;;
  esac
done

if [[ -z "$VERSION" ]]; then
  echo "ERROR: VERSION is required (e.g. v0.1.0-alpha.2)" >&2
  usage
  exit 1
fi

REPO="phin3has/lazyargo"
BASE="https://github.com/${REPO}/releases/download/${VERSION}"

uname_s="$(uname -s)"
uname_m="$(uname -m)"

case "$uname_s" in
  Linux)   OS=linux ;;
  Darwin)  OS=darwin ;;
  *) echo "Unsupported OS: $uname_s" >&2; exit 1 ;;
esac

case "$uname_m" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo "Unsupported arch: $uname_m" >&2; exit 1 ;;
esac

ASSET="lazyargo_${VERSION}_${OS}_${ARCH}.tar.gz"
CHECKSUMS="checksums.txt"

TMP="$(mktemp -d)"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT

mkdir -p "$BIN_DIR"

curl -fsSL "${BASE}/${CHECKSUMS}" -o "$TMP/${CHECKSUMS}"
curl -fsSL "${BASE}/${ASSET}" -o "$TMP/${ASSET}"

# checksum verification
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$TMP" && sha256sum -c "$CHECKSUMS" --ignore-missing)
elif command -v shasum >/dev/null 2>&1; then
  # macOS fallback
  (cd "$TMP" && shasum -a 256 -c "$CHECKSUMS")
else
  echo "ERROR: need sha256sum or shasum" >&2
  exit 1
fi

# extract and install
mkdir -p "$TMP/extract"
tar -xzf "$TMP/${ASSET}" -C "$TMP/extract"

SRC_BIN="$TMP/extract/lazyargo_${VERSION}_${OS}_${ARCH}/lazyargo"
if [[ ! -f "$SRC_BIN" ]]; then
  echo "ERROR: expected binary not found at $SRC_BIN" >&2
  exit 1
fi

install -m 0755 "$SRC_BIN" "$BIN_DIR/lazyargo"

echo "Installed lazyargo to: $BIN_DIR/lazyargo"
"$BIN_DIR/lazyargo" --version || true
