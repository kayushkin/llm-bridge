#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$REPO_DIR"

export PATH="$HOME/.local/share/mise/shims:$(go env GOBIN 2>/dev/null || echo "$HOME/go/bin"):$PATH"

# ── TypeScript type drift check ──────────────────────────────────────────────
# Regenerate TS types and fail if the committed version is stale.
#
# The generated file embeds the current git HEAD SHA in its header. That SHA
# will always differ after a fresh commit (the committed file is stamped with
# its parent commit, regeneration stamps with the new HEAD), so the drift
# check ignores the stamp line and only compares logical content.

echo "==> Checking TypeScript types are up to date..."

OUTFILE="ts/msg.ts"

if [ ! -f "$OUTFILE" ]; then
  echo "ERROR: $OUTFILE does not exist. Run ./generate-ts.sh first."
  exit 1
fi

# Save current generated file, regenerate, compare (ignoring stamp line).
BEFORE=$(mktemp)
cp "$OUTFILE" "$BEFORE"

./generate-ts.sh

if ! diff -q <(grep -v '^// Generated from github.com/kayushkin/llm-bridge/msg @' "$BEFORE") \
             <(grep -v '^// Generated from github.com/kayushkin/llm-bridge/msg @' "$OUTFILE") >/dev/null 2>&1; then
  echo "ERROR: TypeScript types are out of date with Go source."
  echo "       Run ./generate-ts.sh, commit the result, and try again."
  diff --unified <(grep -v '^// Generated from github.com/kayushkin/llm-bridge/msg @' "$BEFORE") \
                 <(grep -v '^// Generated from github.com/kayushkin/llm-bridge/msg @' "$OUTFILE") || true
  rm -f "$BEFORE"
  exit 1
fi

# Stamp-only change — restore the committed file so the git-state check below
# doesn't flag it as a dirty working tree.
mv "$BEFORE" "$OUTFILE"
echo "    TypeScript types are up to date."

# ── Python type drift check ─────────────────────────────────────────────────
# Regenerate Python types and fail if the committed version is stale.
# Same stamp caveat as above — ignore the stamp line during comparison.

echo "==> Checking Python types are up to date..."

PY_OUTFILE="py/llm_bridge_types/msg.py"

if [ ! -f "$PY_OUTFILE" ]; then
  echo "ERROR: $PY_OUTFILE does not exist. Run ./generate-py.sh first."
  exit 1
fi

PY_BEFORE=$(mktemp)
cp "$PY_OUTFILE" "$PY_BEFORE"

./generate-py.sh

if ! diff -q <(grep -v '^# Generated from github.com/kayushkin/llm-bridge/msg @' "$PY_BEFORE") \
             <(grep -v '^# Generated from github.com/kayushkin/llm-bridge/msg @' "$PY_OUTFILE") >/dev/null 2>&1; then
  echo "ERROR: Python types are out of date with Go source."
  echo "       Run ./generate-py.sh, commit the result, and try again."
  diff --unified <(grep -v '^# Generated from github.com/kayushkin/llm-bridge/msg @' "$PY_BEFORE") \
                 <(grep -v '^# Generated from github.com/kayushkin/llm-bridge/msg @' "$PY_OUTFILE") || true
  rm -f "$PY_BEFORE"
  exit 1
fi

# Stamp-only change — restore the committed file so the git-state check below
# doesn't flag it as a dirty working tree.
mv "$PY_BEFORE" "$PY_OUTFILE"
echo "    Python types are up to date."

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

# ── Version stamp info ────────────────────────────────────────────────────────

STAMPED_SHA=$(head -1 "$OUTFILE" | grep -oP '@ \K[a-f0-9]+' || echo "unknown")
echo "    Types generated from commit ${STAMPED_SHA:0:7}."
echo "==> All checks passed. llm-bridge is ready for downstream consumption."
