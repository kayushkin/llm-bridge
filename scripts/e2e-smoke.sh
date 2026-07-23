#!/usr/bin/env bash
# Boot-and-answer smoke test for llm-bridge.
#
# llm-bridge is the shared library (msg/ + bridge/) that every harness and
# provider in the fleet imports; it ships no HTTP server and no long-lived
# process, so the "boot and answer" contract lands on its one executable:
# `cmd/genpy`, the code generator that turns the Go types in msg/ into the
# Python dataclasses at py/llm_bridge_types/msg.py (`go run ./cmd/genpy`).
#
# A green `go build ./...` proves genpy COMPILES. It proves nothing about the
# one thing genpy exists to do: emit a Python source file that is actually valid
# Python. genpy walks the msg/ AST at runtime (go/parser + go/ast) and prints
# dataclasses, type aliases and consts by hand with fmt.Printf. None of that is
# type-checked against Python:
#   - a runtime panic in the AST walk (an unhandled ast node, a nil deref) never
#     shows up in `go build`/`go vet`; only running main() hits it;
#   - a mis-emitted line — a broken class header, a field mapped to an undefined
#     name, a stray token — compiles green on the Go side and ships invalid
#     Python that only fails when something tries to import it;
#   - genpy resolves the msg/ package by the RELATIVE path filepath.Join("msg"),
#     so it only works when run from the repo root; run anywhere else it exits 1
#     with "open msg: no such file or directory". That relative-path contract is
#     invisible to the compiler too.
#
# What is asserted, because genpy's real output is invisible to the compiler:
#   1. genpy exits 0 when run from the repo root and prints a non-empty program.
#   2. that program PARSES AS PYTHON — the whole of stdout is fed to
#      `python3 -c 'ast.parse(...)'`. This is the assertion `go build` cannot
#      make: the generator's product is well-formed Python, not just Go that
#      compiled. It catches a mis-emitted line anywhere in the ~1800-line output.
#   3. the expected header and the core dataclasses (Event, Conversation,
#      Message, StoredSession, AgentDef) are present — genpy walked the msg/
#      package and emitted its foundational types, not an empty/degenerate file.
#   4. FAIL-LOUD: run from a directory with no msg/ subdir, genpy exits non-zero
#      and names the missing msg package. A genpy that silently emitted an empty
#      file from the wrong CWD (the single-source-of-truth failure this fleet
#      guards against) fails here.
#
# HERMETICITY. genpy only READS msg/ and WRITES to stdout — it opens no network
# socket, binds no port, and creates no files (every emit is fmt.Print to
# stdout). So unlike the server and CLI smokes in this fleet there is no live DB,
# port, or credential store it could corrupt. The run still uses a throwaway
# temp dir for the built binary and the captured output, and a curated PATH, so
# nothing leaks into the checkout.
#
# Exits 0 on success, non-zero on the first failing assertion. On failure the
# captured command output is dumped to stderr.
#
# Tunables:
#   E2E_KEEP — set to "1" to leave $TMP_DIR around after the run.

set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BIN_NAME="genpy"

for tool in go python3; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "ERROR: required tool '$tool' not found on PATH" >&2
    exit 2
  fi
done

TMP_DIR="$(mktemp -d -t llm-bridge-e2e.XXXXXX)"
BIN_DIR="$TMP_DIR/bin"
# A directory that deliberately has NO msg/ subdir, for the fail-loud check.
NOMSG_DIR="$TMP_DIR/nomsg"
BIN="$BIN_DIR/$BIN_NAME"
OUT="$TMP_DIR/msg.py"
ERR="$TMP_DIR/err.txt"
mkdir -p "$BIN_DIR" "$NOMSG_DIR"

DUMPED=0
dump_out() {
  [ "$DUMPED" = "1" ] && return 0
  DUMPED=1
  if [ -s "$ERR" ]; then
    echo "----- stderr -----" >&2
    cat "$ERR" >&2
    echo "------------------" >&2
  fi
  if [ -s "$OUT" ]; then
    echo "----- first 20 lines of generated output -----" >&2
    head -20 "$OUT" >&2
    echo "----------------------------------------------" >&2
  fi
}

cleanup() {
  local status=$?
  [ "$status" -ne 0 ] && dump_out
  if [ "${E2E_KEEP:-}" = "1" ]; then
    echo "[e2e] keeping $TMP_DIR"
  else
    rm -rf "$TMP_DIR"
  fi
  return "$status"
}
trap cleanup EXIT INT TERM

step() { printf '\n==> %s\n' "$*"; }
fail() {
  echo "FAIL: $*" >&2
  dump_out
  exit 1
}

step "build $BIN_NAME from $REPO_DIR"
cd "$REPO_DIR"
# genpy imports only the Go standard library (go/ast, go/parser, ...) — pure Go.
# CGO_ENABLED=0 makes any accidental cgo dependency fail at the build step
# instead of shipping a binary that compiles green and dies at runtime (the
# FTS5-class failure this guard exists to catch elsewhere in the fleet).
CGO_ENABLED=0 go build -o "$BIN" ./cmd/genpy
echo "    binary: $BIN ($(ls -lh "$BIN" | awk '{print $5}'))"

step "$BIN_NAME emits a program from the repo root, exit 0"
# genpy resolves msg/ by a RELATIVE path, so it must run with CWD = repo root.
if ! env PATH="/usr/bin:/bin" "$BIN" >"$OUT" 2>"$ERR"; then
  fail "'$BIN_NAME' exited non-zero from the repo root — it could not generate"
fi
[ -s "$OUT" ] || fail "'$BIN_NAME' produced no output"
LINES="$(wc -l <"$OUT")"
echo "    generated $LINES lines"

step "the generated output PARSES AS PYTHON"
# The assertion `go build` cannot make: genpy's product is well-formed Python.
# ast.parse compiles the whole file to an AST without importing anything, so a
# single mis-emitted line anywhere in the output fails here.
if ! python3 -c 'import ast, sys; ast.parse(open(sys.argv[1]).read(), sys.argv[1])' "$OUT" 2>"$ERR"; then
  fail "'$BIN_NAME' emitted output that is not valid Python"
fi
echo "    python3 ast.parse accepted the generated module"

step "the expected header and core dataclasses are present"
grep -q '^from __future__ import annotations' "$OUT" \
  || fail "generated output is missing the expected module header"
for cls in Event Conversation Message StoredSession AgentDef; do
  grep -qE "^class ${cls}:" "$OUT" \
    || fail "generated output is missing the core dataclass 'class ${cls}' — genpy did not emit the msg/ types"
done
echo "    header + core dataclasses (Event, Conversation, Message, StoredSession, AgentDef) present"

step "$BIN_NAME fails LOUDLY from a directory with no msg/ package"
# Run from a temp dir that has no msg/ subdir. genpy must exit non-zero and name
# the missing package, not silently emit an empty file from the wrong CWD.
set +e
( cd "$NOMSG_DIR" && env PATH="/usr/bin:/bin" "$BIN" >"$TMP_DIR/nomsg.out" 2>"$ERR" )
rc=$?
set -e
[ "$rc" -ne 0 ] || fail "'$BIN_NAME' exited 0 from a dir with no msg/ — it did not fail loudly on a missing source package"
grep -qi "msg" "$ERR" \
  || fail "the failure does not name the missing msg package: $(cat "$ERR")"
echo "    failed loudly from the wrong CWD: $(head -1 "$ERR")"

step "SUCCESS — $BIN_NAME boots and generates valid Python from msg/"
