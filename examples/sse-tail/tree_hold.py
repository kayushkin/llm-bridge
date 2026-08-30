#!/usr/bin/env python3
"""One run at a time per working tree, for any instrument that mutates a tree and
reads a whole-tree signal back.

Card `d869d2be-149e-4f99-bf71-2c2c692c595f`. The rule it enforces was paid for by card
`11dda74f`: a sabotage scorer writes a mutation into a repo, runs the suite, and reads
the SUITE'S EXIT CODE as the verdict for that mutation. The exit code belongs to the
tree, not to the mutation. So a second run mutating the same tree hands the first one a
red suite it did not cause, and the first one records it as CAUGHT.

That direction matters. In a reach control the collision produces a false UNREACHED ->
REACHED, which overstates coverage. In a sabotage scorer it produces a false CAUGHT,
which **inflates the score** — and those scores are the numbers every nightly write-up
quotes.

## Why this is a separate module from `reach_control`

`reach_control.exclusive_hold_on_tree` already did this job, but it cannot be the
canonical one, for two reasons and the second is the trap:

  1. It lives in `~/.nightly-shared/`, and the ~20 sabotage engines committed *inside*
     repos cannot import from there (card `bf079200`). A repo's scorer has to be able to
     run on a clone of that repo, so the helper has to be committable next to it.

  2. ⚠️ **Its lock directory is derived from its own file's location** —
     `os.path.dirname(os.path.abspath(__file__)) + "/.reach-control-holds"`. That is
     correct while exactly one copy exists. Copy the module into `<repo>/scripts/` and
     the copy locks in `<repo>/scripts/.reach-control-holds`, which is a DIFFERENT
     namespace per repo. Twenty copies would then hold twenty private locks, interlock
     with nothing, and every one of them would look like it was working. A hold that
     silently protects nothing is worse than no hold, because it stops anyone looking.

So the lock directory here is anchored to `$HOME` and never to `__file__`. Every copy of
this file, wherever it sits, names the same lock for the same tree.

## Contract

    with exclusive_hold_on_tree(repo) as refusal:
        if refusal:
            ...        # refusal is a sentence naming the other holder
            return
        ...            # the tree is ours for the duration of the block

It never raises for contention, and it never waits. Waiting would turn a scorer that
takes 6 minutes into one that takes 12 and looks hung; refusing lets the caller say so
and exit. The caller decides what a refusal means — for a scorer it means "emit no rows",
because a row emitted here is a number a reader would take for a measurement.

Stdlib only, single file, no package. Copy it verbatim into `<repo>/scripts/` and import
it from a sibling scorer. Keep the copies byte-identical: `python3 tree_hold.py --digest`
prints the content hash, so drift between copies is one command away from visible.
"""
import contextlib
import errno
import hashlib
import json
import os
import sys
import time

# Anchored to $HOME, deliberately NOT to __file__. See the trap in the module docstring:
# a __file__-relative directory gives every vendored copy its own private namespace.
# The environment variable exists so a control set can point the holds at a scratch
# directory without racing the real ones; nothing else should set it.
LOCK_DIRECTORY = os.environ.get(
    "NIGHTLY_TREE_HOLD_DIRECTORY",
    os.path.join(os.path.expanduser("~"), ".nightly-tree-holds"))


def hold_path(tree):
    """The lock file for `tree`.

    Keyed on the RESOLVED path so two spellings of one tree — a symlink, a trailing
    slash, a relative path — collide as they should. The digest is the same derivation
    `reach_control` uses, so a reach control run and a sabotage run on one tree land on
    one file and interlock with each other rather than merely with their own kind.
    """
    key = hashlib.sha256(os.path.realpath(tree).encode()).hexdigest()[:16]
    return os.path.join(LOCK_DIRECTORY, key + ".hold")


def _holder_is_alive(pid):
    """Is the process that wrote a hold still running?

    A hold whose writer is gone is litter from a killed run. Treating it as live would
    fail closed forever, which is the one way this refusal could be worse than the
    defect it prevents.
    """
    if not isinstance(pid, int):
        return False
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        # Someone else's process, so it exists. Not ours to reclaim.
        return True
    except OSError:
        return True
    return True


def _read_hold(path):
    try:
        with open(path, encoding="utf-8") as handle:
            return json.load(handle)
    except (OSError, ValueError):
        return None


def describe_holder(tree):
    """What holds `tree` right now, or None. For reporting; never for deciding.

    Deciding on this would be a check-then-act race. `exclusive_hold_on_tree` decides
    with O_EXCL, which is the only channel that cannot be raced.
    """
    return _read_hold(hold_path(tree))


@contextlib.contextmanager
def exclusive_hold_on_tree(tree, purpose=""):
    """Hold `tree` for the duration of the block.

    Yields None when the hold is ours, or a sentence naming the other holder when it is
    not. Never raises for contention, never waits.
    """
    os.makedirs(LOCK_DIRECTORY, exist_ok=True)
    path = hold_path(tree)
    resolved = os.path.realpath(tree)
    ours = json.dumps({"pid": os.getpid(), "tree": resolved,
                       "purpose": purpose or os.path.basename(sys.argv[0] or "?"),
                       "taken_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())})
    for attempt in (1, 2):
        try:
            descriptor = os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o644)
        except OSError as err:
            if err.errno != errno.EEXIST:
                raise
            held = _read_hold(path) or {}
            holder = held.get("pid")
            if attempt == 1 and not _holder_is_alive(holder):
                # Litter from a run that was killed. Reclaim it once. If a second
                # reclaimer wins the race we refuse on the next attempt, which is safe:
                # refusing is always sound, only failing closed forever is not.
                with contextlib.suppress(OSError):
                    os.unlink(path)
                continue
            yield (f"another run holds {resolved} (pid {holder}, "
                   f"{held.get('purpose', 'unknown')}, taken {held.get('taken_at', '?')})"
                   " — its mutations would be read as this run's verdicts")
            return
        try:
            with os.fdopen(descriptor, "w") as handle:
                handle.write(ours)
            yield None
        finally:
            # Only ever remove a hold we still own. A reclaimer may have taken this
            # path while we ran; unlinking then would evict a live holder.
            current = _read_hold(path) or {}
            if current.get("pid") == os.getpid():
                with contextlib.suppress(OSError):
                    os.unlink(path)
        return


def digest():
    """Content hash of this file, so drift between vendored copies is visible."""
    with open(os.path.abspath(__file__), "rb") as handle:
        return hashlib.sha256(handle.read()).hexdigest()[:16]


if __name__ == "__main__":
    if "--digest" in sys.argv:
        print(digest())
    else:
        print(__doc__)
