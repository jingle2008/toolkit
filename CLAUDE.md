<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **toolkit**. Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> If any GitNexus tool warns the index is stale, run `npx gitnexus analyze` in terminal first.

<!-- gitnexus:end -->

<!-- Hand-maintained: keep below the gitnexus markers so `npx gitnexus analyze` can't overwrite it. -->

## Using GitNexus

Impact analysis is a judgment call, not a blanket requirement.

- **Run `gitnexus_impact({target: "symbolName", direction: "upstream"})` before non-trivial edits** — renames, splits, moves, functions with >3 callers, or anything touching a shared helper. Report the blast radius (direct callers, affected processes, risk level) to the user.
- **Run `gitnexus_detect_changes()` before commits that touch 3+ files of behavioral code**, to verify the change only affects expected symbols and execution flows.
- **Skip both** for docs/CHANGELOG edits, style fixes, one-line string changes, test-only edits, formatting/import cleanups, and functions with 0–1 callers where the blast radius is obvious from a 10-second read.
- **Interpret HIGH/CRITICAL as positional, not dangerous.** Risk labels track call-graph fan-out, so a behavior-preserving refactor of a central symbol scores HIGH while still being low risk. Judge by what the change actually does; verify via unchanged signatures plus the invariant-locking tests. Report the label, but frame it as positional.
- Indirect calls through function-typed package variables (the `*Fn` test seams in `internal/resolve/`) are invisible to the call graph — fall back to grep when tracing through a seam.

## Never Do

- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis without saying why the change is safe.
- NEVER rename symbols with find-and-replace — use `gitnexus_rename`, which understands the call graph.
