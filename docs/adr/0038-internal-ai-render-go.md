# ADR 0038: Decisions from `internal/ai/render.go`

Status: Accepted

These decisions were written as comments in `internal/ai/render.go` and moved here so the source no longer carries them. Each entry is the original comment.

## This file holds small, dependency-free helpers shared by every AI

Removed from `internal/ai/render.go:5`.

~~~~text
This file holds small, dependency-free helpers shared by every AI
feature's FormatForTelegram function (Governor, Fleet Commander,
Economy/Research/Battle/Guild/Galaxy/NPC advisors, Dev Console).
All nine of those packages already import internal/ai for
WasTruncated/SanitizeJSONControlChars (ADR-015/016), so this is a
natural shared home instead of duplicating the same six functions
nine times over.

IMPORTANT: every field on a Recommendation - Summary, action/target/
reason strings, warnings, the raw fallback text, etc. - originates
from an LLM's response text, not from our own game logic. That text
is NOT guaranteed to be free of literal "<", ">", or "&" (a model
can very plausibly write something like "trigger when HP < 30%").
Every one of those fields MUST go through HTMLEscape before being
wrapped in a tag, or Telegram will reject the whole message with a
"can't parse entities" 400 error the first time a model happens to
use a comparison operator in its own words.
~~~~


## HTMLBold/HTMLItalic/HTMLCode/HTMLPre wrap already-escaped text in the

Removed from `internal/ai/render.go:30`.

~~~~text
HTMLBold/HTMLItalic/HTMLCode/HTMLPre wrap already-escaped text in the
matching Telegram HTML tag. HTMLPre renders as a full monospace block
(Telegram's equivalent of a markdown ``` fence, which isn't itself a
recognized HTML-mode tag) - used for raw/fallback AI text dumps.
~~~~


## HTMLExpandableQuote wraps already-escaped (possibly multi-line) text

Removed from `internal/ai/render.go:49`.

~~~~text
HTMLExpandableQuote wraps already-escaped (possibly multi-line) text
in Telegram's collapsible "expandable_blockquote" (Bot API 7.3+):
the client shows it collapsed with a tap-to-expand affordance,
rather than dumping the whole thing inline. Built specifically for
AI-generated narrative/reasoning fields, which can legitimately run
to several sentences and would otherwise read as one dense wall of
text sitting in the middle of an otherwise scannable report.

Telegram doesn't take a separate "preview" string - the whole
element is one blockquote that's either collapsed or expanded - so
there's nothing to summarize here beyond passing the full text.
~~~~


## HTMLTable renders a small monospace table wrapped in HTMLPre -

Removed from `internal/ai/render.go:64`.

PROJECT_MASTER_PLAN.md records the shared HTML table helper as ADR-027. The comment cited ADR-026, which is the PgBouncer retry. The Telegram monospace-table constraints in the comment still match HTMLTable.

~~~~text
HTMLTable renders a small monospace table wrapped in HTMLPre -
Telegram has no native table entity, so <pre> (fixed-width font,
whitespace preserved) is the only way to get columns to actually
line up. See ADR-026 in PROJECT_MASTER_PLAN.md for why this exists
as a shared helper rather than each caller hand-padding strings.

Deliberately narrow: Telegram mobile clients wrap monospace text at
roughly 34-40 characters depending on font size and device width,
past which a "table" just becomes a staircase of wrapped lines -
worse than no table at all. Column widths here are computed from
the actual header/cell content (so a table of short values doesn't
pad out to some fixed width for no reason), and any single cell
longer than maxCellWidth is truncated with an ellipsis rather than
being left to blow out the whole layout - callers should pass
already-short values (unit names, percentages, counts), not prose.

headers and each row must be the same length, or the row is
skipped rather than producing a misaligned table (silently wrong
output is worse than an incomplete one here).

IMPORTANT for callers: clip any user-controlled cell content to a
safe rune length yourself BEFORE calling ai.HTMLEscape on it, not
after. If you escape first and let this function's own clipping cut
the (now-escaped) string, it can slice straight through the middle
of an entity like "&amp;", producing invalid HTML that Telegram
will reject the entire message for. This function's own clipping
exists as a backstop against unexpectedly long content, not as the
primary truncation point for anything that needed escaping.
~~~~


## keeps a 2-3 column table under Telegram's mobile wrap width

Removed from `internal/ai/render.go:93`.

~~~~text
keeps a 2-3 column table under Telegram's mobile wrap width
~~~~


## malformed row - drop rather than misalign every column after it

Removed from `internal/ai/render.go:116`.

~~~~text
malformed row - drop rather than misalign every column after it
~~~~
