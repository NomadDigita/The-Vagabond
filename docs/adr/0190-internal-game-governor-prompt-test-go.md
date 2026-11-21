# ADR 0190: Decisions from `internal/game/governor/prompt_test.go`

Status: Accepted

These decisions were written as comments in `internal/game/governor/prompt_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Shuffle module order to confirm the prompt sorts them, which

Removed from `internal/game/governor/prompt_test.go:37`.

~~~~text
Shuffle module order to confirm the prompt sorts them, which
matters for internal/ai's cache key hashing (identical logical
state must produce an identical prompt string).
~~~~


## The next two tests reproduce a real production failure (see

Removed from `internal/game/governor/prompt_test.go:104`.

~~~~text
The next two tests reproduce a real production failure (see
PROJECT_MASTER_PLAN.md ADR-016 / §1.9): a response that got cut off
mid-object (hit MaxTokens) used to be indistinguishable from a
response that never contained JSON at all, both showing the player
the same generic "couldn't parse" message.
~~~~


## The next two tests reproduce a real production failure (see (2)

Removed from `internal/game/governor/prompt_test.go:133`.

~~~~text
The next two tests reproduce a real production failure (see
PROJECT_MASTER_PLAN.md ADR-015 / §1.8): real providers, despite an
explicit "JSON only" instruction, sometimes wrap the object in
prose or leave a raw newline inside a string value. Both used to
make json.Unmarshal fail outright, sending the player a raw JSON
dump instead of a formatted report.
~~~~
