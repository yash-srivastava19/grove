#!/usr/bin/env bash
set -e

REPO="https://github.com/yash-srivastava19/grove"
BIN="${GROVE_BIN:-/usr/local/bin}"

need() { command -v "$1" &>/dev/null || { echo "error: $1 not found"; exit 1; }; }
need go
need git

echo "→ cloning grove..."
TMP=$(mktemp -d)
git clone --depth 1 "$REPO" "$TMP/grove" 2>/dev/null

echo "→ building..."
cd "$TMP/grove"
go build -o grove .

echo "→ installing to $BIN/grove..."
if [ -w "$BIN" ]; then
  mv grove "$BIN/grove"
else
  sudo mv grove "$BIN/grove"
fi

rm -rf "$TMP"

echo ""
echo "✓ grove installed!"
echo ""
echo "  grove           open TUI"
echo "  grove today     open today's daily note"
echo "  grove add 'thought'  quick capture"
echo ""
echo "Run 'grove' to get started."
