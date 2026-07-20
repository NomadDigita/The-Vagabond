# MMO Warfare, Exploration, and Logistics Plan

Status: Phase 1 implementation in progress (2026-07-20).

## Current Architecture Baseline

The game already has a persistent raid lifecycle in `raids` and `raid_forces`: players stage units in `campaign_drafts`, launch a march, arrive into `engaged`, resolve combat over ticks, enter `returning`, and then survivors plus loot return home. The tick engine also already applies weather modifiers, rations/ammunition attrition, co-op rallying, world boss travel, exploration sites, and notifications. This plan extends those systems instead of replacing them.

## Design Goals

1. Make every offensive action feel like a real expedition: staging, route planning, supply packing, travel incidents, battle, looting, and return risk.
2. Require discovery before aggression: no player or AI civilization should be freely attackable until discovered by exploration or route encounter.
3. Treat AI factions as world residents, not isolated training dummies: AI can occupy coordinates, hold resources, fight, expand, and participate in discovery.
4. Make logistics matter: armies need combat units, transport, travel capability, electricity, rations, ammunition, and reinforcement paths.
5. Notify every affected player when discovery, road contact, battle, weather delay, supply failure, or return arrival occurs.
6. Keep all work incremental and recoverable so another developer can resume after any phase.

## Phase 1 — Logistics Floor and Battle-Lifecycle Hardening

Milestones:
- Add database fields for expedition-carried electricity, metal/logistics supplies, and all major loot resources.
- Enforce minimum battle composition: at least 20 combat units, at least one resource transport unit, and at least one travel unit when the army contains walking ground units.
- Make launch supply costs scale with mobilized army size instead of a flat fuel-only fee.
- Increase speed-up cost so forced acceleration is a premium emergency action.
- Extend return payload handling and notifications to include rations, electricity, hydrogen, neuro cores, and dollars; Crystal remains the rarest by applying the lowest loot percentage.
- Update raid radar text to expose supply status and logistics warnings.

Implementation notes:
- Combat units for the minimum floor are Soldiers, Mechs, Destroyers, Bombers, Liberators, Wraiths, Battlecruisers, and Doomsday Rigs.
- Buggies, Ships, and Jets count as travel units. Haulers, Tankers, and Cargo Ships count as resource transport units, but because current draft UI only mobilizes some mobile units, Phase 1 allows owned home-base logistics vehicles to satisfy the transport requirement and deducts campaign supplies from home resources.
- Initial carried supply pools remain stored on `raids` to avoid creating disconnected expedition state. Additional carried resources are added as nullable/defaulted columns.

## Phase 2 — Discovery-Gated Targeting

Milestones:
- Add `encampment_discoveries` to track who has discovered which outpost/AI faction, how it was discovered, and when.
- Filter Tactical Target Matrix to discovered targets only.
- Convert `/explore` from site-only rewards into chance-based discovery of player outposts and AI civilizations in the same/adjacent regions.
- Block direct launch if the defender has not been discovered.
- Record route discoveries when a marching/returning army passes near another base.

Edge cases:
- Self-discovery is implicit and should not require rows.
- Existing live worlds need a compatibility strategy: admins may seed discoveries, or players can rediscover naturally.
- Stealth routes should reduce route-discovery chance but never remove it completely.

## Phase 3 — Road Encounters and Return-Risk Battles

Milestones:
- Add `road_encounters` with two parties, location metadata, response windows, and outcome state.
- During outbound and return ticks, roll encounter checks against discovered/undiscovered nearby armies or bases along the route.
- Notify both parties immediately with options: attack, continue, or ignore until timeout.
- If either party attacks, resolve a field battle using the same battle report renderer and permit theft of carried resources from the defeated marching army.
- Update Expedition Radar with paused/encounter status.

Edge cases:
- If one army is already destroyed by another event, the encounter must close safely.
- Defenders should not receive normal incoming-raid notifications because of a road encounter unless the original target is within radar warning range.

## Phase 4 — Weather, Floods, Camps, and Reinforcement Logistics

Milestones:
- Add route weather events (flood, blizzard, heatwave, sandstorm, EMP, solar flare) with pause durations.
- Support temporary camps: paused armies have a camp state, revised ETA, and supply drain profile.
- Add reinforcement dispatches from home base using dedicated resource units.
- If electricity/logistics supplies deplete, affected high-tech units stop functioning; if food/ammo deplete, the force pauses or retreats depending on severity.
- Make speed-up during weather/camp pauses very expensive and capped.

Balancing assumptions:
- Flood pauses should usually last 12–36 hours in world time unless sped up.
- Speed-up should cost premium-level resources and scale with remaining pause duration.

## Phase 5 — AI Civilizations as Full World Actors

Milestones:
- Replace the single hard-coded Rogue Drone Nest target with seeded AI factions distributed across coordinates and regions.
- Give AI factions resources, workshops, research, expansion ticks, gathering jobs, and raid intent.
- Make AI factions discoverable, attackable, and able to discover/attack humans and other AI.
- Generate notifications for AI discovery, AI scouting, incoming AI raids, AI expansion, and AI defeats.

Edge cases:
- AI should use the same composition rules as humans: minimum 20 combat units, transport, travel, and supplies.
- AI cannot have free omniscience; it must discover before raiding.

## Phase 6 — UI, Balancing, and Observability

Milestones:
- Add a dedicated Logistics Planner panel showing required supplies before launch.
- Add route map/status panels with travel phase, weather, road encounters, supplies, and carried loot.
- Add admin/dev-console balance queries for expedition loss rates, starvation, road battle frequency, Crystal theft, and speed-up spending.
- Add tests for composition requirements, discovery gates, loot multi-resource settlement, and weather pauses.

## Completed This Session

- Phase 1 schema support added via `migrations/027_mmo_warfare_logistics_phase1.sql` and startup migrations: raids now track carried electricity/logistics supplies plus rations, electricity, hydrogen, neuro core, and dollar loot payloads.
- Launch validation now requires at least 20 combat units, at least one resource transport unit, and at least one travel unit when walking units are deployed.
- Launch costs now scale by army size and consume electricity, rations, metal logistics, and hydrogen where advanced vehicles are used.
- Emergency speed-up is intentionally expensive: 2,500 Scrap, $750, and 25 Crystal.
- Active expedition attrition now drains carried electricity and logistics supplies in addition to rations and ammunition; threshold notifications and critical forced retreat cover both food/ammo and power/logistics failures.
- Raid victories now loot multiple resources: Scrap, Metal, rare Crystal, Rations, Electricity, Hydrogen, Neuro Cores, and Dollars. Crystal and Neuro Cores use lower loot multipliers to preserve rarity and value.
- Agent electricity upkeep increased from 0.2 to 2.0 base Electricity per tick before research/mutation reductions. Design opinion: 0.2 was too small for MMO logistics because automation should feel powerful but operationally expensive.
