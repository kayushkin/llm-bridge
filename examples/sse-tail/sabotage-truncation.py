#!/usr/bin/env python3
"""Score the rune-boundary truncation tests by sabotage.

A test suite is a claim that it would notice a defect. Running it against
correct code does not test that claim. This puts each defect back, one at a
time, and records whether the suite went red.

Verdicts
--------
CAUGHT      an assertion fired, or the tests drove production code into a panic
GUARD ONLY  only a reach-guard fired: the fixture broke, not the code under
            test. Scored as NOT caught — crediting it would inflate the score
            by counting the suite detecting itself.
UNNOTICED   the suite stayed green. For a known-negative row that is the
            required result; for a real mutation it is a coverage gap.
VOID        the package did not build, so the run measured nothing. Never a
            pass: a compile error hides whether any test would have caught the
            behaviour.

Run from the directory holding main.go:  python3 sabotage-truncation.py
"""

import os
import re
import subprocess
import sys

SOURCE = "main.go"
TEST_FILE = "main_test.go"
PACKAGE = "./"

# The marker the test file prefixes onto every reach-guard failure. Keep in
# sync with the reachGuard const in main_test.go.
REACH_GUARD = "REACH-GUARD: "


class Mutation:
    """One defect put back into the source.

    `real` rows are defects the suite must catch. `real=False` rows are
    known-negatives: behaviour-preserving rewrites that must go UNNOTICED. A
    scorer with no known-negatives cannot tell a suite that detects the defect
    from one that fails whenever the source changes at all.
    """

    def __init__(self, name, old, new, real, note):
        self.name = name
        self.old = old
        self.new = new
        self.real = real
        self.note = note


MUTATIONS = [
    Mutation(
        "walk-back never runs",
        "for cut > 0 && !utf8.RuneStart(s[cut]) {",
        "for cut > len(s) && !utf8.RuneStart(s[cut]) {",
        True,
        "The original defect, written as a drifted comparison rather than a "
        "deletion. Deleting the loop would orphan the utf8 import, and go test "
        "runs vet, so the row would report a compile error instead of a score.",
    ),
    Mutation(
        "walk-back runs the wrong way",
        "for cut > 0 && !utf8.RuneStart(s[cut]) {",
        "for cut > 0 && utf8.RuneStart(s[cut]) {",
        True,
        "Inverted test: walks back off a boundary onto one, i.e. cuts mid-rune "
        "whenever the plain byte cut would not have.",
    ),
    Mutation(
        "trims to nothing",
        "return s[:cut]",
        "return s[:cut*0]",
        True,
        "Returns the empty string, which is valid UTF-8 and within budget, so "
        "only the length pin can catch it. cut stays referenced, so nothing is "
        "orphaned.",
    ),
    Mutation(
        "negative budget panics again",
        "if maxBytes <= 0 {",
        "if maxBytes <= -1000 {",
        True,
        "Removes the clamp for realistic negative budgets while keeping the "
        "branch live. The suite can only detect this as a panic in production "
        "code, so this row is what exercises the panic verdict from the table "
        "rather than only from the classifier self-test.",
    ),
    Mutation(
        "call site cuts bytes again",
        "return truncateAtRuneBoundary(s, n) + \"…\"",
        "return s[:n] + \"…\"",
        True,
        "Reintroduces the defect at the call site rather than in the helper. A "
        "helper can score full marks while a call site is fixed in appearance "
        "only, so the call site needs its own row.",
    ),
    Mutation(
        "KNOWN NEGATIVE: clamp boundary rewritten",
        "if maxBytes <= 0 {",
        "if maxBytes < 1 {",
        False,
        "Identical for every integer. Must stay UNNOTICED, or the suite is "
        "reacting to the source changing rather than to behaviour changing.",
    ),
    Mutation(
        "KNOWN NEGATIVE: fits-in-budget check rewritten",
        "if len(s) <= maxBytes {",
        "if len(s) < maxBytes+1 {",
        False,
        "Also identical for every integer. Second known-negative, because one "
        "can pass by luck.",
    ),
]


def run_tests():
    """Run the package tests. Returns (built, output)."""
    proc = subprocess.run(
        ["go", "test", "-count=1", PACKAGE],
        capture_output=True,
        text=True,
    )
    out = proc.stdout + proc.stderr
    built = "build failed" not in out and "[build failed]" not in out and not re.search(
        r"^.*\.go:\d+:\d+: ", out, re.M
    )
    return built, out


def failing_tests(output):
    """Test names that went red, so a row reports which test did the catching."""
    return sorted(set(re.findall(r"^\s*--- FAIL: (\S+)", output, re.M)))


def panic_frames(output):
    """Full paths of .go files named in a panic traceback, in order."""
    if "panic:" not in output:
        return []
    return re.findall(r"^\s+(/\S+\.go):\d+", output, re.M)


def classify(output, built):
    """Reduce a test run to one verdict.

    Panics are split by WHERE they happen. A panic whose first frame inside
    this repo is non-test source is the tests driving production code into a
    crash, and that is detection. A panic in the fixture is the test breaking
    itself.

    Frames are matched by FULL PATH, not basename: the runtime's own
    runtime/panic.go ends in .go and is not _test.go, so a basename filter
    promotes every fixture panic to real coverage.
    """
    if not built:
        return "VOID"

    if "panic:" in output:
        repo = os.path.realpath(".")
        for frame in panic_frames(output):
            real = os.path.realpath(frame)
            if not real.startswith(repo + os.sep) and real != repo:
                continue  # stdlib or runtime frame, keep looking
            if real.endswith("_test.go"):
                return "GUARD ONLY"  # the fixture crashed, not the code
            return "CAUGHT"  # production code crashed
        return "GUARD ONLY"  # panic never entered this repo's own code

    if "\nok  " in output or output.strip().endswith("ok"):
        if "FAIL" not in output:
            return "UNNOTICED"

    if "FAIL" not in output:
        return "UNNOTICED"

    guard_fired = REACH_GUARD in output
    # An assertion failure is any red line that is not a reach-guard message.
    assertion_fired = False
    for line in output.splitlines():
        if re.match(r"^\s+\S+\.go:\d+: ", line) and REACH_GUARD not in line:
            assertion_fired = True
            break

    if assertion_fired:
        return "CAUGHT"
    if guard_fired:
        return "GUARD ONLY"
    return "CAUGHT"


def self_test():
    """Drive every verdict from a literal before trusting the classifier.

    A classifier is a new instrument sitting exactly where a wrong answer is
    invisible: one that can never return GUARD ONLY prints a clean score
    forever. Exercising it directly is cheap and does not depend on the suite
    being able to reach each branch.
    """
    repo = os.path.realpath(".")
    src = os.path.join(repo, SOURCE)
    test = os.path.join(repo, TEST_FILE)

    cases = [
        (
            "build failure -> VOID",
            "# github.com/x\n./main.go:12:3: undefined: nope\n",
            False,
            "VOID",
        ),
        (
            "green -> UNNOTICED",
            "ok  \tgithub.com/x\t0.005s\n",
            True,
            "UNNOTICED",
        ),
        (
            "assertion -> CAUGHT",
            "--- FAIL: TestA (0.00s)\n    main_test.go:58: budget=41: result is not valid UTF-8\nFAIL\n",
            True,
            "CAUGHT",
        ),
        (
            "reach-guard alone -> GUARD ONLY",
            "--- FAIL: TestA (0.00s)\n    main_test.go:85: "
            + REACH_GUARD
            + "no budget straddled a rune\nFAIL\n",
            True,
            "GUARD ONLY",
        ),
        (
            "guard + assertion -> CAUGHT",
            "--- FAIL: TestA (0.00s)\n    main_test.go:85: "
            + REACH_GUARD
            + "no budget straddled\n"
            + "    main_test.go:58: budget=41: not valid UTF-8\nFAIL\n",
            True,
            "CAUGHT",
        ),
        (
            "panic in production source -> CAUGHT",
            "panic: runtime error: slice bounds out of range [:-1]\n"
            "\t/usr/lib/go/src/runtime/panic.go:860 +0x13a\n"
            "\t" + src + ":170 +0x6e\n"
            "\t" + test + ":128 +0x38\nFAIL\n",
            True,
            "CAUGHT",
        ),
        (
            "panic in the fixture -> GUARD ONLY",
            "panic: runtime error: index out of range\n"
            "\t/usr/lib/go/src/runtime/panic.go:860 +0x13a\n"
            "\t" + test + ":154 +0x38\nFAIL\n",
            True,
            "GUARD ONLY",
        ),
    ]

    print("classifier self-test")
    seen = set()
    bad = 0
    for name, output, built, want in cases:
        got = classify(output, built)
        seen.add(got)
        mark = "ok " if got == want else "BAD"
        if got != want:
            bad += 1
        print(f"  {mark} {name:42s} -> {got}")

    verdicts = {"CAUGHT", "GUARD ONLY", "UNNOTICED", "VOID"}
    missing = verdicts - seen
    if missing:
        print(f"  BAD unreachable verdicts: {sorted(missing)}")
        bad += 1
    if bad:
        print("\nclassifier is wrong; not scoring anything with a broken instrument")
        sys.exit(1)
    print("  all four verdicts reachable and correct\n")


def main():
    if not os.path.exists(SOURCE) or not os.path.exists(TEST_FILE):
        sys.exit(f"run me from the directory holding {SOURCE} and {TEST_FILE}")

    self_test()

    original = open(SOURCE).read()

    built, out = run_tests()
    if not built:
        sys.exit("baseline does not build:\n" + out)
    if "FAIL" in out:
        sys.exit("baseline is already red; fix that before scoring:\n" + out)
    print("baseline green\n")

    rows = []
    try:
        for m in MUTATIONS:
            if m.old not in original:
                rows.append((m, "STALE PATTERN", []))
                print(f"STALE PATTERN  {m.name}")
                print("               pattern not found in source — a stale regex")
                print("               mutates nothing and scores a bogus UNNOTICED\n")
                continue
            if original.count(m.old) != 1:
                rows.append((m, "AMBIGUOUS PATTERN", []))
                print(f"AMBIGUOUS      {m.name} (matches {original.count(m.old)} sites)\n")
                continue

            open(SOURCE, "w").write(original.replace(m.old, m.new, 1))
            built, out = run_tests()
            verdict = classify(out, built)
            reds = failing_tests(out)
            rows.append((m, verdict, reds))

            want = "CAUGHT" if m.real else "UNNOTICED"
            ok = verdict == want
            print(f"{verdict:14s} {m.name}")
            print(f"               want {want}  {'ok' if ok else '<-- WRONG'}")
            if reds:
                print(f"               red: {', '.join(reds)}")
            print(f"               {m.note}\n")
    finally:
        open(SOURCE, "w").write(original)

    # A clean tree is not the same claim as the fix still being present.
    restored = open(SOURCE).read()
    if restored != original:
        sys.exit("FAILED TO RESTORE the source")
    if "truncateAtRuneBoundary(s, n)" not in restored or "utf8.RuneStart" not in restored:
        sys.exit("the fix is NOT present after the run")
    built, out = run_tests()
    if not built or "FAIL" in out:
        sys.exit("suite is not green after restore:\n" + out)
    print("restored, fix asserted present, suite green\n")

    real = [r for r in rows if r[0].real]
    negs = [r for r in rows if not r[0].real]
    caught = [r for r in real if r[1] == "CAUGHT"]
    held = [r for r in negs if r[1] == "UNNOTICED"]
    guard_only = [r for r in rows if r[1] == "GUARD ONLY"]

    print(f"SCORE  {len(caught)}/{len(real)} real defects caught")
    print(f"       {len(held)}/{len(negs)} known-negatives correctly unnoticed")
    print(f"       {len(guard_only)} rows caught by a reach-guard only")
    bad = [r for r in rows if r[1] in ("STALE PATTERN", "AMBIGUOUS PATTERN")]
    if bad:
        print(f"       {len(bad)} rows scored nothing (stale/ambiguous pattern)")
    sys.exit(0 if len(caught) == len(real) and len(held) == len(negs) and not bad else 1)


if __name__ == "__main__":
    main()
