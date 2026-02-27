#!/usr/bin/env bash
#
# Download and install the latest gho release binary from GitHub.
# No Go toolchain required.
#
# Usage:
#   scripts/install-release.sh [--prefix DIR]
#
# One-liner:
#   curl -fsSL https://raw.githubusercontent.com/AugustDG/ghotto/main/scripts/install-release.sh | bash
#

set -euo pipefail

REPO="AugustDG/ghotto"
BIN_NAME="gho"
PREFIX="/usr/local/bin"

usage() {
  cat <<'EOF'
Download and install the latest gho binary from GitHub releases.

Usage:
  install-release.sh [--prefix DIR]

Options:
  --prefix DIR     Install directory (default: /usr/local/bin)
  -h, --help       Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prefix)
      PREFIX="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

# --- Detect OS ---
detect_os() {
  local os
  os="$(uname -s)"
  case "$os" in
    Darwin|darwin) echo "darwin" ;;
    Linux|linux)   echo "linux" ;;
    *)
      echo "Unsupported OS: $os" >&2
      exit 1
      ;;
  esac
}

# --- Detect architecture ---
detect_arch() {
  local arch
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)   echo "amd64" ;;
    aarch64|arm64)   echo "arm64" ;;
    *)
      echo "Unsupported architecture: $arch" >&2
      exit 1
      ;;
  esac
}

# --- HTTP fetch helper (curl preferred, wget fallback) ---
fetch() {
  local url="$1"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "$url"
  else
    echo "Neither curl nor wget found. Install one and retry." >&2
    exit 1
  fi
}

fetch_to_file() {
  local url="$1"
  local dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$dest" "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$url"
  else
    echo "Neither curl nor wget found. Install one and retry." >&2
    exit 1
  fi
}

# --- Main ---

OS="$(detect_os)"
ARCH="$(detect_arch)"

echo "Detected platform: ${OS}/${ARCH}"

# Fetch latest release tag
echo "Fetching latest release..."
TAG="$(fetch "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | head -1 \
  | sed -E 's/.*"tag_name":[[:space:]]*"([^"]+)".*/\1/')"

if [[ -z "$TAG" ]]; then
  echo "Failed to determine latest release tag." >&2
  echo "Check that https://github.com/${REPO}/releases has published releases." >&2
  exit 1
fi

echo "Latest release: ${TAG}"

# Build download URL
ARTIFACT="${BIN_NAME}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARTIFACT}"

# Download to temp directory
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading ${URL}..."
fetch_to_file "$URL" "${TMP_DIR}/${ARTIFACT}"

# Extract
echo "Extracting..."
tar -xzf "${TMP_DIR}/${ARTIFACT}" -C "$TMP_DIR"

if [[ ! -f "${TMP_DIR}/${BIN_NAME}" ]]; then
  echo "Archive did not contain '${BIN_NAME}' binary." >&2
  exit 1
fi

# Install
DEST_DIR="$(cd "$PREFIX" 2>/dev/null && pwd || echo "$PREFIX")"
DEST_PATH="${DEST_DIR}/${BIN_NAME}"

echo "Installing ${BIN_NAME} to ${DEST_PATH}..."
mkdir -p "$DEST_DIR" 2>/dev/null || true

if [[ -w "$DEST_DIR" ]]; then
  install -m 755 "${TMP_DIR}/${BIN_NAME}" "$DEST_PATH"
else
  sudo install -m 755 "${TMP_DIR}/${BIN_NAME}" "$DEST_PATH"
fi

echo ""
echo "Done. ${BIN_NAME} ${TAG} installed to ${DEST_PATH}"
echo "Run 'gho setup' to configure your model."
