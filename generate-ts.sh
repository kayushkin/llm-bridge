#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$REPO_DIR"

export PATH="$HOME/.local/share/mise/shims:$(go env GOBIN 2>/dev/null || echo "$HOME/go/bin"):$PATH"

COMMIT=$(git rev-parse HEAD)
SHORT=$(git rev-parse --short HEAD)
DIRTY=""
if ! git diff --quiet HEAD -- msg/; then
  DIRTY="-dirty"
fi

echo "==> Generating TypeScript types from Go (commit ${SHORT}${DIRTY})..."
tygo generate

# Prepend version header to generated file.
OUTFILE="ts/msg.ts"
HEADER="// Generated from github.com/kayushkin/llm-bridge/msg @ ${COMMIT}${DIRTY}
// Run ./generate-ts.sh to regenerate.
"

TMPFILE=$(mktemp)
echo "$HEADER" > "$TMPFILE"
cat "$OUTFILE" >> "$TMPFILE"
mv "$TMPFILE" "$OUTFILE"

echo "    wrote $OUTFILE ($(wc -l < "$OUTFILE") lines, commit ${SHORT}${DIRTY})"
