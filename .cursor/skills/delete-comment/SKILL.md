---
name: delete-comment
description: Use when cleaning comments out of a repository, when commentlint reports a new comment, or when a review finds comments that restate the code, excuse a hack, or record a decision.
---

# Delete comment

Remove comments that do not have to be there. Keep the build green by shrinking a comment allowlist, not by adding explanations back into the source.

## Categories

Put every comment into one category, then act:

| Category | What it is | Action |
| --- | --- | --- |
| Tautology | Repeats what the code, name, or signature already shows | Delete it |
| Stale | Names code, files, or behavior that are gone or wrong | Delete it |
| Excuse | Lets a hack, shortcut, or unfinished behavior stay | Delete it and open an issue that describes the hack to fix |
| Decision record | Records a non-obvious decision: a constraint, invariant, or why this shape was chosen | Move the text into `docs/adr/` and delete the comment |

One comment gets one category. If any sentence records a real decision, the comment is a decision record. If it justifies shipping a shortcut, it is an excuse even when it also mentions a plan.

## Keep

Leave a comment in place only when the code or tooling needs it:

- License headers
- Build tags and compiler directives (`//go:`, `//export`, `//line`, `+build`)
- `nolint` lines that a linter still requires
- Shebangs and encoding cookies
- Doc comments a configured linter rejects when they are absent

A doc comment that merely restates the symbol is a tautology. Delete it when no linter requires it. Do not keep a comment because it might be useful later.

## Steps

1. Inventory comments in source files (Go, Python, and SQL in this repo). Ignore text inside strings. Group consecutive comments that form one note.
2. Classify each group. Open the surrounding code before calling a comment stale.
3. Delete tautologies and stale comments.
4. Delete excuses. Open one GitHub issue per hack, with the file and the behavior to fix. Related copies of the same hack can share an issue.
5. Move each decision record into `docs/adr/`. Preserve the original wording. Group notes that are the same decision. Delete the comment from the source.
6. Shrink the comment allowlist. Do not add entries.
7. Build the project and run its tests.

## Ratchet

`tools/commentlint` walks Go, Python, and SQL comments and compares them to `tools/commentlint/baseline.txt`.

- `make comments` and `go test ./tools/commentlint` fail when the tree and the baseline differ.
- `go run ./tools/commentlint --write` rewrites the baseline only when every current comment is already allowed. Use it after a deletion so the ceiling drops.
- `go run ./tools/commentlint --ratchet=origin/main` fails when the baseline has more entries than that ref. A missing baseline on the base ref is the first introduction and is allowed.

New prose comments fail the check. The allowlist count only goes down.

## Do not

- Push the cleanup to the default branch. Open a pull request.
- Restore a deleted comment because the code looks bare.
- File an issue for a decision that was moved into an ADR. File an issue only for an excuse.
- Edit a migration's SQL while deleting its comments. Comments go; statements stay.
