# Vagabond Unicode Asset Audit

Tracked Go sources contain **197** distinct symbol tokens across **8150** occurrences.

| Unicode | Uses | Real source context |
|---|---:|---|
| ━ | 3740 | `"━━━━━━━━━━━━━━━━━━━━━━\n"+` |
| ─ | 1968 | `// ── /ai_status (admin only) ────────────────────────────────────────` |
| ⚠️ | 322 | `btnDBReset := selector.Data("⚠️ Reset Database", "admin_action", "db_reset")` |
| ❌ | 213 | `return c.Send("❌ Access Denied: Authorized administrators only.", keyboards.MainNavigation())` |
| ▰ | 104 | `bar += "▰"` |
| ▱ | 98 | `return "▱▱▱▱▱▱▱▱▱▱"` |
| 🛡️ | 67 | `bot.Handle("🛡️ Clan Alliances", clan.HandleClanPanel)` |
| 🚚 | 50 | `{"cargo_mk1", "🚚", "Cargo Ship Mk I", "cargo_mk1", content.MustFindUnit("cargo_mk1").DeconstructRefund()},` |
| ` | 49 | ``ALTER TABLE clans ADD COLUMN IF NOT EXISTS icon VARCHAR(10) DEFAULT '🏴';`,` |
| ➜ | 46 | `panelText += fmt.Sprintf("⚔️ %s ➜ %s [%s]\n   Loot: ⚙️%.0f 🔩%.0f 💎%.0f\n\n", attName, defName, state, stolenScrap, stolenMetal, stolenCrystal)` |
| 🤖 | 45 | `('The Rustlord', '🤖👹', 500000, 500000, 5000),` |
| ⚙️ | 43 | `('Scrap Titan', '⚙️👹', 1200000, 1200000, 12000),` |
| 🛰️ | 43 | `bot.Handle("🛰️ Server Metrics", admin.HandleAdminMetrics)` |
| ⚔️ | 41 | `bot.Handle("⚔️ Tactical Combat", combat.HandleRaidBoard)` |
| 💎 | 38 | `btnGiftPremium := selector.Data("💎 Gift Premium", "admin_action", "gift_premium")` |
| ⚡ | 37 | `bot.Handle("⚡ Force Master Tick", admin.HandleAdminTick)` |
| 🔩 | 37 | `{"metal_mine", "🔩", "Metal Mine", "Passive Metal generation every tick."},` |
| 🪖 | 35 | `bot.Handle("🪖 Recruit Troops", factory.HandleRecruitPanel)` |
| ✅ | 31 | `btnCancel := confirmSelector.Data("✅ Cancel", "admin_action", "db_reset_cancel")` |
| 📡 | 30 | `bot.Handle("📡 Terminal HQ", onboarding.HandleStart)` |
| ☢️ | 27 | `bot.Handle("☢️ Strategic Silo", silo.HandleSiloPanel)` |
| 🏗️ | 25 | `bot.Handle("🏗️ Infrastructure Grid", camp.HandleInfrastructureGridPanel)` |
| 🚀 | 24 | `_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("🚀 STRIKE FORCE DEPLOYED: 🪖 %d Soldiers, 🤖 %d Mechs marching to engage %s! ETA: %.0f minutes.", soldiers, mechs, bossName,` |
| 👥 | 23 | `bot.Handle("👥 Hero Commander", hero.HandleHeroPanel)` |
| 💀 | 23 | `status = "💀 DEFEATED (respawning...)"` |
| 📋 | 23 | `panelText := "📋━━━━━━━━━━━━━━━━━━━━━━📋\n" +` |
| 🏆 | 22 | `bot.Handle("🏆 Global Ranking", ranking.HandleRankingPanel)` |
| 🎯 | 22 | `{"gauss_cannon", "🎯", "Gauss Cannon", "Heavy anti-armor turret. Strong defense bonus."},` |
| 🧭 | 21 | `bot.Handle("🧭 World Exploration", exploration.HandleExplorePanel)` |
| 🔄 | 21 | `btnRefresh := selector.Data("🔄 Refresh Analysis", "battle_analyst_refresh")` |
| 💥 | 21 | `_ = c.Respond(&telebot.CallbackResponse{Text: "💥 Alliance dissolved!"})` |
| 🏦 | 19 | `bot.Handle("🏦 System Economy", econ.HandleEconPanel)` |
| 🏴 | 18 | ``ALTER TABLE clans ADD COLUMN IF NOT EXISTS icon VARCHAR(10) DEFAULT '🏴';`,` |
| 🔍 | 18 | `panelText += "\n🔍 Browse existing clans with /clans, or found your own:\n" +` |
| 📊 | 18 | `panelText += fmt.Sprintf("\n⚔️ AT WAR with %s!\n📊 Score: %.0f - %.0f\n", activeWarOpponent, warScoreMine, warScoreTheirs)` |
| 👑 | 18 | `reportText += fmt.Sprintf("👑 WARLORD DETECTED: Faction Superpower - %s\n", nest.HeroSuperpower)` |
| 🌐 | 17 | `icon VARCHAR(10) DEFAULT '🌐',` |
| 🧠 | 17 | `bot.Handle("🧠 Automation Agent", agentH.HandleAgent)` |
| 📦 | 17 | `bot.Handle("📦 Warehouse Reserves", econ.HandleWarehouseReserves)` |
| 💰 | 16 | `btnTaxRate := selector.Data("💰 Set Tax Rate", "admin_action", "tax_rate")` |
| 💵 | 16 | `"💵 Available Balance: $%.1f\n\n"+` |
| 🤝 | 16 | `btnCoop := selector.Data(fmt.Sprintf("🤝 Co-Op [%d]", i+1), "stage_coop", t.id)` |
| ⛺ | 15 | `bot.Handle("⛺ Outpost Camp", camp.HandleCamp)` |
| 🎖️ | 15 | `"🎖️ Your Rank: %s\n",` |
| ✈️ | 15 | `"✈️ Jets: %d\n",` |
| ⛏️ | 14 | `bot.Handle("⛏️ Active Mining", camp.HandleActiveMining)` |
| 🚗 | 14 | `bot.Handle("🚗 Logistics Vehicles", factory.HandleVehiclesPanel)` |
| 🔧 | 14 | `"summary": "🔧 PLACEHOLDER (no live AI configured) — set an API key to get a real building/upgrade analysis for this base.",` |
| ↩️ | 14 | `coopAlert := "↩️ ALLIANCE NOTICE: Ongoing campaign has been aborted due to an administrative database reset. Your contributed forces have returned safely."` |
| 🦅 | 14 | `var randomAnimalIcons = []string{"🦅", "🐺", "🐻", "🦁", "🐯", "🦂", "🐍", "🦇", "🦉", "🐗", "🦖", "🦍", "🐉", "🦌", "🦊"}` |
| 📜 | 14 | `"📜━━━━━━━━━━━━━━━━━━━━━━📜\n"+` |
| 👹 | 13 | `emoji VARCHAR(10) DEFAULT '👹',` |
| 👻 | 13 | `"👻 Wraiths: %d / %d active\n"+` |
| 💱 | 12 | `bot.Handle("💱 Market Exchange", exchange.HandleExchangePanel)` |
| 🚢 | 12 | `"🚢👑 Battlecruisers: %d / %d active\n"+` |
| 🌑 | 12 | `"🌑💀 Doomsday Rigs: %d / %d active\n"+` |
| 🌍 | 12 | `_, _ = c.Bot().Edit(msg, fmt.Sprintf("📡 ANALYSIS ENGINE: WEATHER VECTORS...\n[▰▰▰▰▱▱▱▱▱▱] 40%%\n🌍 Weather Status: %s", weatherStatus))` |
| 🧬 | 11 | `bot.Handle("🧬 Mutation Core", camp.HandleMutationsPanel)` |
| 👁️ | 11 | `reportText += fmt.Sprintf("🛡️🤖 Guardians: %d \| 👁️ Observers: %d\n", nest.Guardians, nest.Observers)` |
| 🛩️ | 11 | `"🛩️ Bombers: %d / %d active\n"+` |
| 🏭 | 10 | `bot.Handle("🏭 Heavy Workshop", factory.HandleFactoryPanel)` |
| 🚨 | 10 | `"🚨⚔️ CLAN WAR DECLARED! ⚔️🚨\n\n"+` |
| ⛵ | 10 | `"⛵ Clipper Ships: %d / %d active\n"+` |
| 🕊️ | 10 | `return c.Respond(&telebot.CallbackResponse{Text: "🕊️ DIPLOMATIC IMMUNITY: Your Clan has an active pact with the target's Clan. Break it with /break_pact first if you wish to attack` |
| 💳 | 10 | `"💳 Debt: %.1f Scrap \| $%.1f Cash\n\n"+` |
| 🪙 | 9 | `bot.Handle("🪙 Inject Resources", admin.HandleAdminGive)` |
| ♻️ | 9 | `bot.Handle("♻️ Deconstruct Units", deconstruct.HandleDeconstructPanel)` |
| 🎁 | 9 | `btnGiftResources := selector.Data("🎁 Gift Resources", "admin_action", "gift_resources")` |
| 🛠️ | 9 | `"🛠️ [Collector]: Auto-scavenges +5.0 Scrap, +2.0 Rations per tick.\n"+` |
| ⏳ | 9 | `panelText += fmt.Sprintf("\n⏳ You have %d pending application(s) awaiting Leader approval.\n", pendingCount)` |
| 📬 | 9 | `panelText += fmt.Sprintf("\n📬 %d pending application(s) - review them below!\n", pendingApps)` |
| 📝 | 9 | `return c.Send(fmt.Sprintf("📝 YOUR DESCRIPTION:\n\"%s\"\n\nUsage: /description [text] (max 200 characters)", current))` |
| 🏛️ | 8 | `bot.Handle("🏛️ Admin Terminal", admin.HandleAdminPanel)` |
| ⬅️ | 8 | `bot.Handle("⬅️ Back to HQ", onboarding.HandleStart)` |
| 💡 | 8 | `"💡 This composition is fixed to your current level - grow stronger, and the Nest grows with you.\n" +` |
| 🔮 | 8 | `"🔮━━━━━━━━━━━━━━━━━━━━━━🔮\n"+` |
| ☠️ | 7 | `('Apex Wraith', '☠️👹', 3000000, 3000000, 30000)` |
| 🧪 | 7 | `bot.Handle("🧪 Research Lab", research.HandleResearchPanel)` |
| ✍️ | 7 | `return "✍️ Reply with: `username days`\nExample: `wanderer99 30`"` |
| 🗺️ | 7 | `"🗺️ TACTICAL ROUTING PATHS:\n"+` |
| 🛵 | 7 | `{"scout", "🛵", "Scout Walker", "scouts", content.MustFindUnit("scout").DeconstructRefund()},` |
| 🏟️ | 6 | `bot.Handle("🏟️ Combat Arena", arena.HandleArenaPanel)` |
| 🎈 | 6 | `"🎈 [Pump Hydrogen] — Costs: 15.0 Electricity (+10.0 Hydrogen / miner)\n\n"+` |
| 🚛 | 6 | `{"hauler", "🚛", "Resource Hauler", "haulers", map[string]float64{"metal": 220.0}},` |
| 🏪 | 6 | `"🏪━━━━━━━━━━━━━━━━━━━━━━🏪\n"+` |
| 🚩 | 6 | `"🚩━━━━━━━━━━━━━━━━━━━━━━🚩\n"+` |
| ✊ | 5 | `bot.Handle("✊ The Rebellion", rebellion.HandleRebellionPanel)` |
| 📻 | 5 | `bot.Handle("📻 Wasteland Radio", world.HandleWorldFeed)` |
| 🟢 | 5 | `statusLabel = "🟢 ACTIVE (RUNNING...)"` |
| 🛒 | 5 | `"🛒 MINER SHOP DECK:\n"+` |
| 📉 | 5 | `respText += "\n📉 Supply Crisis: sale prices depressed."` |
| 📍 | 5 | `panelText += fmt.Sprintf("📍 %s (%s)\n   Reward: %s %.0f %s\n\n", name, siteType, rewardEmoji(rewardType), rewardAmount, rewardType)` |
| 🛸 | 4 | `bot.Handle("🛸 Expedition Radar", combat.HandleExpeditionRadar)` |
| 🔴 | 4 | `statusLabel := "🔴 STANDBY (OFFLINE)"` |
| ☀️ | 4 | `{"solar_panel", "☀️", "Solar Panel", "Generates bonus Electricity independent of your Generator."},` |
| 🏅 | 4 | `panelText += fmt.Sprintf("%s %d. 🏴 %s (%d/15) — 🏅 %.0f pts\n", medalFor(rank), rank, name, members, score)` |
| 📨 | 4 | `btn := selector.Data(fmt.Sprintf("📨 Apply to %s", name), "clan_apply", clanID)` |
| 🔢 | 4 | `"🔢 Bulk Step: %s (tap 🔢 Step to cycle x1 → x10 → x100 → MAX)\n"+` |
| 🌧️ | 4 | `weatherStatus = "🌧️ Corrosive precipitation active. Mechs structure structural integrity hazard."` |
| 🌩️ | 4 | `weatherStatus = "🌩️ EMP burst detected. Electronics-dependent systems degraded."` |
| 🥫 | 4 | `"🥫 SURVIVAL MATERIALS:\n"+` |
| ✨ | 4 | `"✨ THE ETHER SHOP ✨\n"+` |
| 🔇 | 4 | `return c.Send("🔇 This player has muted you - your message wasn't delivered.")` |
| ░ | 4 | `rowText += "░░ "` |
| 🔨 | 3 | `bot.Handle("🔨 Structural Upgrades", camp.HandleStructuralUpgrades)` |
| 🔌 | 3 | `toggleLabel = "🔌 Shutdown Agent"` |
| 🔕 | 3 | `subStatus := "🔕 Unsubscribed"` |
| 🔔 | 3 | `toggleAlertLabel := "🔔 Subscribe to Idle Alerts"` |
| 🦾 | 3 | `"🦾 [Cybernetic Salvage Lvl %d / 5] (Cost: 20 Crystal, 5 Neuro)\n"+` |
| ⚒️ | 3 | `"⚒️ /clan_create [name]\n" +` |
| 🚪 | 3 | `btnLeave := selector.Data("🚪 Leave Clan", "leave_clan", clanID.String)` |
| 📏 | 3 | `return c.Send("⚠️ Usage: /clan_create [name]\n📏 3-24 characters.")` |
| 🎉 | 3 | `return c.Send(fmt.Sprintf("🛡️🎉 CLAN ESTABLISHED: \"%s\"! You are its Leader. Use /clans to see it listed, or /clan for your HUD.", name))` |
| 💬 | 3 | `"💬 Text shortcut: /add <n> <unit>, /remove <n> <unit>\n\n"+` |
| 🌪️ | 3 | `weatherStatus = "🌪️ Sandstorm active. Navigation and targeting accuracy reduced."` |
| 🏋️ | 3 | `"🏋️ [Train Commander] — Cost: 50 Scrap (+20 XP)\n"+` |
| 💊 | 3 | `"💊 [Heal Injury] — Cost: 50 Rations (Heals sustained scars)\n"+` |
| 🏠 | 3 | `navCaptionMain    = "🏠 Returning to Outpost HQ..."` |
| 🟠 | 3 | `return "🟠 HIGH"` |
| 💭 | 3 | `fmt.Fprintf(&b, "💭 Reasoning: %s\n", rec.Reasoning)` |
| 💻 | 2 | `"💻 ADMINISTRATIVE METRICS PANEL\n"+` |
| 🧩 | 2 | `"🧩 GC Cycles Executed: %d\n"+` |
| ⛔ | 2 | `return c.Send("⛔ Administrator access required.")` |
| 🟡 | 2 | `{"light_laser", "🟡", "Light Laser", "Cheap early turret. Small defense rating bonus."},` |
| 👤 | 2 | `panelText += fmt.Sprintf("👤 %s (@%s)\n", fName, username)` |
| 📢 | 2 | `broadcast := fmt.Sprintf("📢 %s [%s]:\n\n%s", clanName, sender.FirstName, msg)` |
| 🐲 | 2 | `bossText += fmt.Sprintf("🐲 BOSS ASSAULT (MARCHING): Target: %s\n   ETA: %s (%ds remaining)\n\n", bossName, resolveTime.UTC().Format("15:04:05"), timeLeft)` |
| 🧿 | 2 | `reportText += fmt.Sprintf("🧿 Nuclear Shields: %d\n", nest.Shields)` |
| 🛍️ | 2 | `btnBuy := selector.Data(fmt.Sprintf("🛍️ Buy [%d]", index), "buy_listing", listID)` |
| 🦠 | 2 | `return "🦠 Disease Outbreak - rations consumption elevated."` |
| 📖 | 2 | `btnManual := selector.Data("📖 Survival Manual", "view_manual")` |
| 🎛️ | 2 | `"🎛️ ADVANCED GAMEPLAY SETTINGS 🎛️\n"+` |
| 🗞️ | 2 | `"🗞️ LATEST WASTELAND EVENTS 🗞️\n" +` |
| 💹 | 2 | `researchplanner.GoalEconomy:  "💹 Economy",` |
| ⚖️ | 2 | `researchplanner.GoalBalanced: "⚖️ Balanced",` |
| 🏢 | 2 | `rowText += "🏢 "` |
| 🏳️ | 2 | `fmt.Sprintf("🏳️ %s was already defeated by another survivor before your forces arrived. Turning back - no fight, no losses.", m.bossName))` |
| 🍖 | 2 | `fmt.Sprintf("🍖 RATIONS RUNNING LOW: Your marching force toward [%s] is down to %.0f%% rations.", ex.defenderName, newRations))` |
| 🖥️ | 2 | `b.WriteString("🖥️ AI DEVELOPER CONSOLE\n\n")` |
| 📈 | 2 | `fmt.Fprintf(&b, "📈 Market timing: %s\n", rec.MarketTiming)` |
| 🔀 | 2 | `string(ActionSplit):     "🔀",` |
| ❔ | 2 | `icon := "❔"` |
| 🎭 | 1 | `btnFaction := selector.Data("🎭 Change My Faction", "admin_action", "faction")` |
| 🏷️ | 1 | `"🏷️ License: %s\n"+` |
| 🚫 | 1 | `state = "🚫 disabled"` |
| 🔵 | 1 | `{"ion_cannon", "🔵", "Ion Cannon", "Anti-drone/anti-air turret. Strong defense bonus."},` |
| 🟣 | 1 | `{"plasma_turret", "🟣", "Plasma Turret", "Top-tier turret. Massive defense bonus."},` |
| 🛬 | 1 | `{"hangar", "🛬", "Hangar", "Increases maximum unit capacity (+20 per level)."},` |
| 📶 | 1 | `{"trade_beacon", "📶", "Trade Beacon", "Discounts Ether Shop conversion costs."},` |
| 🐺 | 1 | `var randomAnimalIcons = []string{"🦅", "🐺", "🐻", "🦁", "🐯", "🦂", "🐍", "🦇", "🦉", "🐗", "🦖", "🦍", "🐉", "🦌", "🦊"}` |
| 🐻 | 1 | `var randomAnimalIcons = []string{"🦅", "🐺", "🐻", "🦁", "🐯", "🦂", "🐍", "🦇", "🦉", "🐗", "🦖", "🦍", "🐉", "🦌", "🦊"}` |
| 🦁 | 1 | `var randomAnimalIcons = []string{"🦅", "🐺", "🐻", "🦁", "🐯", "🦂", "🐍", "🦇", "🦉", "🐗", "🦖", "🦍", "🐉", "🦌", "🦊"}` |
| 🐯 | 1 | `var randomAnimalIcons = []string{"🦅", "🐺", "🐻", "🦁", "🐯", "🦂", "🐍", "🦇", "🦉", "🐗", "🦖", "🦍", "🐉", "🦌", "🦊"}` |
| 🦂 | 1 | `var randomAnimalIcons = []string{"🦅", "🐺", "🐻", "🦁", "🐯", "🦂", "🐍", "🦇", "🦉", "🐗", "🦖", "🦍", "🐉", "🦌", "🦊"}` |
| 🐍 | 1 | `var randomAnimalIcons = []string{"🦅", "🐺", "🐻", "🦁", "🐯", "🦂", "🐍", "🦇", "🦉", "🐗", "🦖", "🦍", "🐉", "🦌", "🦊"}` |
| 🦇 | 1 | `var randomAnimalIcons = []string{"🦅", "🐺", "🐻", "🦁", "🐯", "🦂", "🐍", "🦇", "🦉", "🐗", "🦖", "🦍", "🐉", "🦌", "🦊"}` |
| 🦉 | 1 | `var randomAnimalIcons = []string{"🦅", "🐺", "🐻", "🦁", "🐯", "🦂", "🐍", "🦇", "🦉", "🐗", "🦖", "🦍", "🐉", "🦌", "🦊"}` |
| 🐗 | 1 | `var randomAnimalIcons = []string{"🦅", "🐺", "🐻", "🦁", "🐯", "🦂", "🐍", "🦇", "🦉", "🐗", "🦖", "🦍", "🐉", "🦌", "🦊"}` |
| 🦖 | 1 | `var randomAnimalIcons = []string{"🦅", "🐺", "🐻", "🦁", "🐯", "🦂", "🐍", "🦇", "🦉", "🐗", "🦖", "🦍", "🐉", "🦌", "🦊"}` |
| 🦍 | 1 | `var randomAnimalIcons = []string{"🦅", "🐺", "🐻", "🦁", "🐯", "🦂", "🐍", "🦇", "🦉", "🐗", "🦖", "🦍", "🐉", "🦌", "🦊"}` |
| 🐉 | 1 | `var randomAnimalIcons = []string{"🦅", "🐺", "🐻", "🦁", "🐯", "🦂", "🐍", "🦇", "🦉", "🐗", "🦖", "🦍", "🐉", "🦌", "🦊"}` |
| 🦌 | 1 | `var randomAnimalIcons = []string{"🦅", "🐺", "🐻", "🦁", "🐯", "🦂", "🐍", "🦇", "🦉", "🐗", "🦖", "🦍", "🐉", "🦌", "🦊"}` |
| 🦊 | 1 | `var randomAnimalIcons = []string{"🦅", "🐺", "🐻", "🦁", "🐯", "🦂", "🐍", "🦇", "🦉", "🐗", "🦖", "🦍", "🐉", "🦌", "🦊"}` |
| 🔫 | 1 | `reportText += fmt.Sprintf("🔫 Light Laser: Lvl %d\n", nest.LightLaserLvl)` |
| 🔥 | 1 | `reportText += fmt.Sprintf("🔥 Heavy Laser: Lvl %d\n", nest.HeavyLaserLvl)` |
| 🧲 | 1 | `reportText += fmt.Sprintf("🧲 Gauss Cannon: Lvl %d\n", nest.GaussCannonLvl)` |
| ☄️ | 1 | `reportText += fmt.Sprintf("☄️ Plasma Turret: Lvl %d\n", nest.PlasmaTurretLvl)` |
| 💔 | 1 | `return c.Send(fmt.Sprintf("🕊️💔 Diplomatic pact with %s has been broken. Raids between your Clans are no longer blocked.", targetName))` |
| ⚗️ | 1 | `"⚗️ CONVERSION DEALS:\n",` |
| 🕵️ | 1 | `"🛰️ [Tactical Drone] ➜ 🔩100 Metal, 💎10 Crystal ➜ 🕵️ Spy Satellite / 🚨 Interceptor\n"+` |
| 🏰 | 1 | `"🛩️ [Bomber] ➜ 🔩1300 Metal, 💎60 Crystal ➜ 🏰 Hard-counters Turrets\n"+` |
| 🛢️ | 1 | `"🚛 Resource Haulers: %d \| 🛢️ Fuel Tankers: %d \| 🔧 Recovery Rigs: %d\n"+` |
| 🔒 | 1 | `return c.Send(fmt.Sprintf("⚠️ Usage: /fed_found [name]\n💰 Cost: %.0f Crystal\n🔒 Only your Clan's King can found a Federation.", federationFoundCost))` |
| 🎗️ | 1 | `"🎗️ Psychological Trait: %s\n"+` |
| 🩺 | 1 | `"🩺 Injuries Sustained: %s\n"+` |
| 🌀 | 1 | `return c.Send(fmt.Sprintf("🌀✨ TELEPORT COMPLETE! Your outpost now stands at [%d, %d] in a %s biome.", newX, newY, biome))` |
| 🏕️ | 1 | `navCaptionCamp    = "🏕️ Accessing Outpost Command..."` |
| ✏️ | 1 | `"✏️ RENAME OUTPOST\n\nUsage: /name [new name]\n\n💰 Cost: %.0f Crystal + $%.0f\n📏 3-20 characters, letters/numbers/spaces/hyphens only.\n\n⚠️ This changes your public display name e` |
| 🔑 | 1 | `"🔑 Your Referral Code: %s\n"+` |
| 🔊 | 1 | `return c.Send(fmt.Sprintf("🔊 %s has been unmuted.", targetUsername))` |
| 🌎 | 1 | `"🌎 ECONOMY-WIDE TOTALS:\n"+` |
| 🥇 | 1 | `return "🥇"` |
| 🥈 | 1 | `return "🥈"` |
| 🥉 | 1 | `return "🥉"` |
| 🌟 | 1 | `return "🌟", "Rebel Hero"` |
| 🔰 | 1 | `return "🔰", "Recruit"` |
| 🩹 | 1 | `{"integrity", "integrity_tech_lvl", "🩹", "Integrity", "Reduces casualties suffered by your units in combat."},` |
| 📰 | 1 | `newsText += "📰 LATEST SECTOR BROADCAST REPORTS:\n"` |
| ⌛ | 1 | `log.Println("⌛ Processing master game tick pass...")` |
| 📎 | 1 | `fmt.Fprintf(&b, "%d. %s\n   📎 %s\n   💡 %s\n", i+1, p.Observation, p.Evidence, p.Suggestion)` |
| 🚁 | 1 | `// "37🚁 Bomber. 14💥 Destroyer." Units with a zero count are omitted.` |
| 🆕 | 1 | `fmt.Fprintf(&b, "🆕 New players: %s\n\n", rec.NewPlayerNarrative)` |
| 🚧 | 1 | `fmt.Fprintf(&b, "🚧 Bottlenecks: %s\n", rec.Bottlenecks)` |
| 🔭 | 1 | `string(ActionScout):     "🔭",` |
| 🌌 | 1 | `b.WriteString("🌌 AI DYNAMIC GALAXY ADVISOR\n\n")` |
| ⬆️ | 1 | `icon = "⬆️"` |
| ⬇️ | 1 | `icon = "⬇️"` |
