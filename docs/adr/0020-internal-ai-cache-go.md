# ADR 0020: Decisions from `internal/ai/cache.go`

Status: Accepted

These decisions were written as comments in `internal/ai/cache.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Cache is the abstraction the Service uses to avoid re-paying for

Removed from `internal/ai/cache.go:11`.

~~~~text
Cache is the abstraction the Service uses to avoid re-paying for
identical AI calls. The default implementation is an in-process TTL
map; a future session may swap this for a Redis- or Postgres-backed
implementation without touching Service.
~~~~


## CacheKey derives a stable cache key from the parts of a request that

Removed from `internal/ai/cache.go:20`.

~~~~text
CacheKey derives a stable cache key from the parts of a request that
affect its output. Feature and UserID are included so one player's
cached answer is never served to another under a different context.
~~~~
