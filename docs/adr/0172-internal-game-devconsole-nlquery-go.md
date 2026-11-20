# ADR 0172: Decisions from `internal/game/devconsole/nlquery.go`

Status: Accepted

These decisions were written as comments in `internal/game/devconsole/nlquery.go` and moved here so the source no longer carries them. Each entry is the original comment.

## This file implements natural-language admin queries: an admin types

Removed from `internal/game/devconsole/nlquery.go:1`.

~~~~text
This file implements natural-language admin queries: an admin types
a free-text question, and gets a grounded answer. See queries.go's
doc comment for the safety model this depends on — the model never
writes or executes SQL; it only ever picks a name from a fixed
whitelist plus two bounded integers.

This is a two-call flow, unlike every other Phase B-J feature's
single call:
 1. Classify: given the admin's question and the whitelist
    (IntentDescriptions), the model returns {"intent": "...",
    "days": N, "limit": N}. Validated against the whitelist and
    clamped to safe bounds before anything runs.
 2. Answer: given the admin's original question and the actual
    query result text (never anything the model wrote), the model
    produces a final natural-language answer grounded in those real
    numbers.

Two calls cost more than one, but there's no safe way to compress
this to one call without either letting the model's raw output
drive execution (unsafe) or answering before the real numbers exist
(ungrounded/hallucination-prone).
~~~~


## NLQueryMemoryScope is used for both calls in the natural-language

Removed from `internal/game/devconsole/nlquery.go:33`.

~~~~text
NLQueryMemoryScope is used for both calls in the natural-language
query flow, distinct from MemoryScope (the weekly-report scope).
~~~~


## parseClassification decodes the classifier's response text,

Removed from `internal/game/devconsole/nlquery.go:89`.

~~~~text
parseClassification decodes the classifier's response text,
tolerating a markdown code fence the same way every other Phase B-J
package does.
~~~~


## Ask runs the full classify → execute → answer flow for one

Removed from `internal/game/devconsole/nlquery.go:170`.

~~~~text
Ask runs the full classify → execute → answer flow for one
free-text admin question. It stores all turns in ai_memory under
NLQueryMemoryScope.

Read-only end to end: every intent in queryIntents is a SELECT (see
queries.go), and nothing the model outputs is ever used as
executable code — only as a validated choice from a fixed list plus
two clamped integers.
~~~~
