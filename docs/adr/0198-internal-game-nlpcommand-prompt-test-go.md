# ADR 0198: Decisions from `internal/game/nlpcommand/prompt_test.go`

Status: Accepted

These decisions were written as comments in `internal/game/nlpcommand/prompt_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestParseResponse_BarterListing covers the market's extension beyond

Removed from `internal/game/nlpcommand/prompt_test.go:75`.

~~~~text
TestParseResponse_BarterListing covers the market's extension beyond
cash-only listings: "list 200k scrap for 40k metal" should parse to
ask_type="metal", not force a dollar price.
~~~~


## TestParseResponse_BuyMarketItem covers the 2026-08-05 addition:

Removed from `internal/game/nlpcommand/prompt_test.go:99`.

~~~~text
TestParseResponse_BuyMarketItem covers the 2026-08-05 addition:
"buy 200 metal for $500" should parse to ActionBuyMarketItem with
min_quantity/max_dollars, not be confused with list_market_item.
~~~~


## TestParseResponse_LaunchRaid covers the 2026-08-05 addition:

Removed from `internal/game/nlpcommand/prompt_test.go:128`.

~~~~text
TestParseResponse_LaunchRaid covers the 2026-08-05 addition:
"raid Lotus Dominion" should parse to ActionLaunchRaid with
target_name, and must NOT require confirmation - the card/confirm
step lives inside the existing draft-board UI this jumps to, not
here (see ActionLaunchRaid's doc comment in prompt.go).
~~~~


## The old dollars-only field name should be gone entirely -

Removed from `internal/game/nlpcommand/prompt_test.go:174`.

~~~~text
The old dollars-only field name should be gone entirely -
this tool now expresses cash via ask_type="dollars" instead
of a dedicated price field.
~~~~
