#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Build and install gho.

Usage:
  scripts/install-common.sh [--prefix DIR] [--bin-name NAME]

Options:
  --prefix DIR     Install directory (default: /usr/local/bin)
  --bin-name NAME  Installed binary name (default: gho)
  -h, --help       Show this help
EOF
}

PREFIX="/usr/local/bin"
BIN_NAME="gho"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --shell)
      # Accepted for compat with install.zsh wrapper, ignored
      shift 2
      ;;
    --prefix)
      PREFIX="${2:-}"
      shift 2
      ;;
    --bin-name)
      BIN_NAME="${2:-}"
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

if ! command -v go >/dev/null 2>&1; then
  echo "go is required but not found in PATH" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

echo "Building gho..."
go build -o gho .

DEST_DIR="$(cd "$PREFIX" 2>/dev/null && pwd || true)"
if [[ -z "$DEST_DIR" ]]; then
  DEST_DIR="$PREFIX"
fi
DEST_PATH="$DEST_DIR/$BIN_NAME"

echo "Installing ${BIN_NAME} to ${DEST_PATH}..."
mkdir -p "$DEST_DIR" 2>/dev/null || true
if [[ -w "$DEST_DIR" ]]; then
  install -m 755 gho "$DEST_PATH"
else
  sudo install -m 755 gho "$DEST_PATH"
fi

echo "Done. Run 'gho setup' to configure your model."
