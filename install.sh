#!/bin/sh
set -e

REPO="nebari-dev/skillsctl"
INSTALL_DIR="/usr/local/bin"
FALLBACK_DIR="$HOME/.local/bin"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux) OS="linux" ;;
  darwin) OS="darwin" ;;
  mingw*|msys*|cygwin*)
    echo "Windows detected. Download from https://github.com/$REPO/releases"
    echo "Or run: go install github.com/$REPO/cli@latest"
    exit 1
    ;;
  *)
    echo "Unsupported OS: $OS"
    exit 1
    ;;
esac

# Detect arch
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# Fetch latest version
echo "Fetching latest release..."
VERSION=$(curl -sL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$VERSION" ]; then
  echo "Failed to fetch latest version. Check https://github.com/$REPO/releases"
  exit 1
fi
VERSION_NUM="${VERSION#v}"

echo "Installing skillsctl $VERSION for ${OS}/${ARCH}..."

# Download
ARCHIVE="skillsctl_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/${VERSION}/${ARCHIVE}"
CHECKSUMS_URL="https://github.com/$REPO/releases/download/${VERSION}/checksums.txt"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

curl -sL "$URL" -o "$TMPDIR/$ARCHIVE" || {
  echo "Download failed: $URL"
  exit 1
}
curl -sL "$CHECKSUMS_URL" -o "$TMPDIR/checksums.txt" || {
  echo "Failed to download checksums"
  exit 1
}

# Verify checksum
EXPECTED=$(grep "$ARCHIVE" "$TMPDIR/checksums.txt" | awk '{print $1}')
if [ -z "$EXPECTED" ]; then
  echo "Archive not found in checksums file"
  exit 1
fi

if command -v sha256sum > /dev/null 2>&1; then
  ACTUAL=$(sha256sum "$TMPDIR/$ARCHIVE" | awk '{print $1}')
elif command -v shasum > /dev/null 2>&1; then
  ACTUAL=$(shasum -a 256 "$TMPDIR/$ARCHIVE" | awk '{print $1}')
else
  echo "Warning: no sha256sum or shasum found, skipping checksum verification"
  ACTUAL="$EXPECTED"
fi

if [ "$ACTUAL" != "$EXPECTED" ]; then
  echo "Checksum mismatch!"
  echo "  Expected: $EXPECTED"
  echo "  Got:      $ACTUAL"
  exit 1
fi

# Extract
tar -xzf "$TMPDIR/$ARCHIVE" -C "$TMPDIR"

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMPDIR/skillsctl" "$INSTALL_DIR/skillsctl"
  echo "Installed to $INSTALL_DIR/skillsctl"
else
  mkdir -p "$FALLBACK_DIR"
  mv "$TMPDIR/skillsctl" "$FALLBACK_DIR/skillsctl"
  echo "Installed to $FALLBACK_DIR/skillsctl"
  case ":$PATH:" in
    *":$FALLBACK_DIR:"*) ;;
    *) echo "Add $FALLBACK_DIR to your PATH: export PATH=\"$FALLBACK_DIR:\$PATH\"" ;;
  esac
fi

# Verify
skillsctl --version 2>/dev/null || echo "Install complete. Run 'skillsctl --version' to verify."
