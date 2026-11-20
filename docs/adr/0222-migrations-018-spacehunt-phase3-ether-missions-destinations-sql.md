# ADR 0222: Decisions from `migrations/018_spacehunt_phase3_ether_missions_destinations.sql`

Status: Accepted

These decisions were written as comments in `migrations/018_spacehunt_phase3_ether_missions_destinations.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## Ether trickles in passively (resource.go), scaled by Technology

Removed from `migrations/018_spacehunt_phase3_ether_missions_destinations.sql:12`.

~~~~text
Ether trickles in passively (resource.go), scaled by Technology
research (0.02 per tick per research level) until the Technology
Center building lands in a later phase.

Commands added: /ether (Ether Shop - convert Ether into Metal, Crystal,
Scrap, Neuro Cores, or Cash), /missions (consolidated view of active
raids, World Boss engagements, and mining queues), /destinations
(previously-scouted rival outposts + the always-available Rogue Drone
Nest - matches SpaceHunt's 'map of discovered planets')
~~~~
