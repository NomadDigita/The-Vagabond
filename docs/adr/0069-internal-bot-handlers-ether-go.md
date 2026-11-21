# ADR 0069: Decisions from `internal/bot/handlers/ether.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/ether.go` and moved here so the source no longer carries them. Each entry is the original comment.

## HandleEtherShop (/ether) converts Ether - a slow-trickling passive

Removed from `internal/bot/handlers/ether.go:39`.

~~~~text
HandleEtherShop (/ether) converts Ether - a slow-trickling passive
resource - into a burst of other goods, matching SpaceHunt's Ether
Shop feature.
~~~~
