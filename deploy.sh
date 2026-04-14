#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$REPO_DIR"

export PATH="$HOME/.local/share/mise/shims:$(go env GOBIN 2>/dev/null || echo "$HOME/go/bin"):$PATH"

# ── TypeScript type drift check ──────────────────────────────────────────────
# Regenerate TS types and fail if the committed version is stale.

echo "==> Checking TypeScript types are up to date..."

OUTFILE="ts/msg.ts"

if [ ! -f "$OUTFILE" ]; then
  echo "ERROR: $OUTFILE does not exist. Run ./generate-ts.sh first."
  exit 1
fi

# Save current generated file, regenerate, compare.
BEFORE=$(mktemp)
cp "$OUTFILE" "$BEFORE"

./generate-ts.sh

if ! diff -q "$BEFORE" "$OUTFILE" >/dev/null 2>&1; then
  echo "ERROR: TypeScript types are out of date with Go source."
  echo "       Run ./generate-ts.sh, commit the result, and try again."
  diff --unified "$BEFORE" "$OUTFILE" || true
  rm -f "$BEFORE"
  exit 1
fi

rm -f "$BEFORE"
echo "    TypeScript types are up to date."

# ── Ensure changes are committed and pushed ──────────────────────────────────

echo "==> Checking git state..."

if ! git diff --quiet HEAD; then
  echo "ERROR: Uncommitted changes. Commit and push before deploying."
  exit 1
fi

BRANCH=$(git rev-parse --abbrev-ref HEAD)
REMOTE_REF="origin/$BRANCH"

git fetch origin "$BRANCH" --quiet 2>/dev/null || true

LOCAL=$(git rev-parse HEAD)
REMOTE=$(git rev-parse "$REMOTE_REF" 2>/dev/null || echo "")

if [ "$LOCAL" != "$REMOTE" ]; then
  echo "ERROR: Local HEAD ($LOCAL) differs from $REMOTE_REF (${REMOTE:-not found})."
  echo "       Push your changes before deploying."
  exit 1
fi

echo "    All changes committed and pushed."

# ── Version stamp check ──────────────────────────────────────────────────────

COMMITTED_SHA=$(head -1 "$OUTFILE" | grep -oP '@ \K[a-f0-9]+' || echo "")
HEAD_SHA=$(git rev-parse HEAD)

if [ "$COMMITTED_SHA" != "$HEAD_SHA" ]; then
  echo "ERROR: TypeScript types stamped at commit $COMMITTED_SHA but HEAD is $HEAD_SHA."
  echo "       Run ./generate-ts.sh, commit, and push."
  exit 1
fi

echo "    Version stamp matches HEAD ($HEAD_SHA)."
echo "==> All checks passed. llm-bridge is ready for downstream consumption."
