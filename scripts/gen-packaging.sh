#!/usr/bin/env bash
set -euo pipefail

# Generates packaging helpers from a given release tag by reading checksums.txt.
# Output:
# - Homebrew formula snippet
# - Scoop manifest snippet

REPO="phin3has/lazyargo"
VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
  echo "Usage: gen-packaging.sh vX.Y.Z[-prerelease]" >&2
  exit 1
fi

BASE="https://github.com/${REPO}/releases/download/${VERSION}"
CHECKSUMS_URL="${BASE}/checksums.txt"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "$CHECKSUMS_URL" -o "$TMP/checksums.txt"

sha_for() {
  local filename="$1"
  awk -v f="$filename" '$2==f{print $1}' "$TMP/checksums.txt"
}

LINUX_AMD64_SHA="$(sha_for "lazyargo_${VERSION}_linux_amd64.tar.gz")"
DARWIN_AMD64_SHA="$(sha_for "lazyargo_${VERSION}_darwin_amd64.tar.gz")"
DARWIN_ARM64_SHA="$(sha_for "lazyargo_${VERSION}_darwin_arm64.tar.gz")"
LINUX_ARM64_SHA="$(sha_for "lazyargo_${VERSION}_linux_arm64.tar.gz")"
WIN_AMD64_SHA="$(sha_for "lazyargo_${VERSION}_windows_amd64.zip")"

cat <<EOF
# --- Homebrew (tap) formula snippet ---
# Create a tap repo (recommended): github.com/phin3has/homebrew-tap
# Then add: Formula/lazyargo.rb

class Lazyargo < Formula
  desc "lazygit-style TUI for Argo CD"
  homepage "https://github.com/${REPO}"
  version "${VERSION#v}"

  on_macos do
    on_arm do
      url "${BASE}/lazyargo_${VERSION}_darwin_arm64.tar.gz"
      sha256 "${DARWIN_ARM64_SHA}"
    end
    on_intel do
      url "${BASE}/lazyargo_${VERSION}_darwin_amd64.tar.gz"
      sha256 "${DARWIN_AMD64_SHA}"
    end
  end

  on_linux do
    on_arm do
      url "${BASE}/lazyargo_${VERSION}_linux_arm64.tar.gz"
      sha256 "${LINUX_ARM64_SHA}"
    end
    on_intel do
      url "${BASE}/lazyargo_${VERSION}_linux_amd64.tar.gz"
      sha256 "${LINUX_AMD64_SHA}"
    end
  end

  def install
    bin.install "lazyargo_${VERSION}_#{OS.kernel_name}_#{Hardware::CPU.arch}/lazyargo" => "lazyargo"
  end

  test do
    system "#{bin}/lazyargo", "--version"
  end
end

# --- Scoop manifest snippet ---
{
  "version": "${VERSION#v}",
  "description": "lazygit-style TUI for Argo CD",
  "homepage": "https://github.com/${REPO}",
  "license": "MIT",
  "architecture": {
    "64bit": {
      "url": "${BASE}/lazyargo_${VERSION}_windows_amd64.zip",
      "hash": "${WIN_AMD64_SHA}"
    }
  },
  "bin": "lazyargo_${VERSION}_windows_amd64\\lazyargo.exe",
  "checkver": "github",
  "autoupdate": {
    "architecture": {
      "64bit": {
        "url": "https://github.com/${REPO}/releases/download/v\$version/lazyargo_v\$version_windows_amd64.zip"
      }
    },
    "hash": {
      "url": "https://github.com/${REPO}/releases/download/v\$version/checksums.txt",
      "find": "(?m)^([a-fA-F0-9]{64})\\s+lazyargo_v\$version_windows_amd64\\.zip$"
    }
  }
}
EOF
