# ADR 0219: Decisions from `migrations/011_spacehunt_slice9_march_timing.sql`

Status: Accepted

These decisions were written as comments in `migrations/011_spacehunt_slice9_march_timing.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## Stores the real outbound march duration so the return trip can be

Removed from `migrations/011_spacehunt_slice9_march_timing.sql:10`.

~~~~text
Stores the real outbound march duration so the return trip can be
anchored to it (previously the return trip used a flat, disconnected
15-minute base regardless of how far the raid actually traveled).
~~~~
