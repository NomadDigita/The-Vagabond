# ADR 0109: Decisions from `internal/bot/handlers/spawnlocation_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/spawnlocation_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestRandomContinent_AlwaysValid ensures randomContinent (used by

Removed from `internal/bot/handlers/spawnlocation_test.go:41`.

~~~~text
TestRandomContinent_AlwaysValid ensures randomContinent (used by
/newjobteleport and /ghostprotocol) only ever returns one of the four
canonical continents - the bug being fixed is the literal string
"Unknown Sector", which matched none of world.Continents and silently
excluded a relocated base from every per-continent system forever.
~~~~


## TestFlavorCountriesByContinent_CoversEveryContinent ensures every

Removed from `internal/bot/handlers/spawnlocation_test.go:86`.

~~~~text
TestFlavorCountriesByContinent_CoversEveryContinent ensures every
continent in world.Continents has at least one flavor country
defined, so locationDescriptor never silently falls back to "an
uncharted territory" for a real, valid continent.
~~~~


## TestOutpostNameRegex_ValidatesConsistentlyWithRename ensures the

Removed from `internal/bot/handlers/spawnlocation_test.go:98`.

~~~~text
TestOutpostNameRegex_ValidatesConsistentlyWithRename ensures the
naming rule shared between the mandatory first-naming prompt
(HandleOnboardingPendingInput) and the paid /name rename command
(HandleRenameOutpost) behaves as documented: letters, numbers,
spaces, and hyphens only.
~~~~


## TestLocationDescriptor_NoRawHTMLFromUserInput is a documentation-style

Removed from `internal/bot/handlers/spawnlocation_test.go:119`.

~~~~text
TestLocationDescriptor_NoRawHTMLFromUserInput is a documentation-style
guard: locationDescriptor's inputs are always either a fixed continent
name or drawn from the fixed flavorTowns/flavorCountriesByContinent
pools, never player-authored text, so the rendered string should never
contain characters that would need htmlEscape.
~~~~
