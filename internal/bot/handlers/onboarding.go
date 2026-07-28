package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
	"github.com/NomadDigita/The-Vagabond/internal/models"
	"gopkg.in/telebot.v3"
)

type OnboardingHandler struct {
	DB *sql.DB
}

func NewOnboardingHandler(db *sql.DB) *OnboardingHandler {
	return &OnboardingHandler{DB: db}
}

// generateReferralCode derives a referral code directly from a player's
// Telegram ID. Because Telegram IDs are themselves unique, base36-encoding
// the ID guarantees a collision-free code with no DB round-trip or retry
// loop needed (unlike the old `telegramID % 1_000_000` scheme, which could
// map two different players to the same code).
func generateReferralCode(telegramID int64) string {
	return "REF" + strings.ToUpper(strconv.FormatInt(telegramID, 36))
}

// referralMilestones defines one-time bonus payouts unlocked once a
// referrer's total referral count reaches the given threshold, on top of
// the standard per-referral reward.
var referralMilestones = []struct {
	Count                 int
	Metal, Crystal, Neuro float64
}{
	{5, 25000, 250, 25000},
	{10, 75000, 750, 75000},
	{25, 250000, 2500, 250000},
}

func (h *OnboardingHandler) HandleStart(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	if sender == nil {
		return errors.New("sender details missing from context")
	}

	ctx := context.Background()

	var user models.User
	queryUser := `SELECT telegram_id, username, first_name, state, COALESCE(faction, '') FROM users WHERE telegram_id = $1`
	err := h.DB.QueryRowContext(ctx, queryUser, sender.ID).Scan(&user.TelegramID, &user.Username, &user.FirstName, &user.State, &user.Faction)

	if err == nil {
		if user.Faction == "" {
			return h.renderFactionChoice(c, sender.ID)
		}

		var camp models.Encampment
		var res models.Resources
		var region string
		var myX, myY int

		queryCamp := `
			SELECT e.id, e.name, r.scrap, r.rations, r.electricity, c.region, c.x, c.y 
			FROM encampments e
			JOIN resources r ON r.encampment_id = e.id
			JOIN coordinates c ON c.id = e.coordinate_id
			WHERE e.user_id = $1`

		err = h.DB.QueryRowContext(ctx, queryCamp, user.TelegramID).Scan(&camp.ID, &camp.Name, &res.Scrap, &res.Rations, &res.Electricity, &region, &myX, &myY)
		if err != nil {
			log.Printf("Failed to query existing player details: %v", err)
			return c.Send("⚠️ System error reclaiming session database.", keyboards.MainNavigation())
		}

		_ = h.DB.QueryRowContext(ctx, "UPDATE users SET last_active = CURRENT_TIMESTAMP WHERE telegram_id = $1", user.TelegramID)

		var activeMiners int
		_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(SUM(miners_assigned), 0) FROM active_mining_queues WHERE encampment_id = $1 AND is_completed = FALSE", camp.ID).Scan(&activeMiners)

		var ownedMiners int
		_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(miners, 1) FROM workshop_inventory WHERE encampment_id = $1", camp.ID).Scan(&ownedMiners)

		var outboundCount int
		_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM raids WHERE attacker_id = $1 AND (state = 'marching' OR state = 'engaged')", camp.ID).Scan(&outboundCount)

		var inboundExists bool
		_ = h.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM raids WHERE defender_id = $1 AND state = 'marching')", camp.ID).Scan(&inboundExists)

		systemState := "🟢 SECURE (NOMINAL)"
		if inboundExists {
			systemState = "🔴 WARNING (HOSTILE RAID IN TRANSIT)"
		}

		dashboard := fmt.Sprintf(
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"📡 VAGABOND SYSTEM TERMINAL\n"+
				"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"Welcome back, Commander %s.\n\n"+
				"SYSTEM TELEMETRY HUD:\n"+
				"📡 State: %s\n"+
				"⛏️ Miners: %d / %d active | Idle: %d\n"+
				"🚀 Transits: %d outbound transits running\n\n"+
				"Faction: %s\n"+
				"🌍 Territory: Encampment located in [%s]\n"+
				"⛺ Encampment: %s\n"+
				"📍 Location: [X: %d, Y: %d]\n\n"+
				"CURRENT RESOURCE BALANCES:\n"+
				"⚙️ Scrap: %.1f\n"+
				"🥫 Rations: %.1f\n"+
				"⚡ Electricity Cells: %.1f\n"+
				"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"Use the command manual below to learn terminal shortcuts.",
			user.FirstName, systemState, activeMiners, ownedMiners, ownedMiners-activeMiners, outboundCount,
			formatFactionLabel(user.Faction), region, camp.Name, myX, myY, res.Scrap, res.Rations, res.Electricity,
		)

		selector := &telebot.ReplyMarkup{}
		btnWarehouse := selector.Data("📦 Warehouse Stocks", "view_warehouse")
		btnManual := selector.Data("📖 Survival Manual", "view_manual")

		selector.Inline(
			selector.Row(btnWarehouse, btnManual),
		)

		return sendPanelWithNav(c, navCaptionMain, keyboards.MainNavigation(), dashboard, selector)
	}

	if !errors.Is(err, sql.ErrNoRows) {
		log.Printf("Database check execution failure: %v", err)
		return c.Send("⚠️ Database reading failure.", keyboards.MainNavigation())
	}

	// Brand new user - capture a referral code from the /start payload if
	// present (e.g. /start REF123456), before the faction picker.
	if refCode := strings.TrimSpace(c.Message().Payload); refCode != "" {
		var referrerID int64
		if refErr := h.DB.QueryRowContext(ctx, "SELECT telegram_id FROM users WHERE referral_code = $1", refCode).Scan(&referrerID); refErr == nil && referrerID != sender.ID {
			_, _ = h.DB.ExecContext(ctx, `
				INSERT INTO users (telegram_id, username, first_name, state, referred_by) 
				VALUES ($1, $2, $3, 'onboarding', $4)
				ON CONFLICT (telegram_id) DO NOTHING`, sender.ID, sender.Username, sender.FirstName, referrerID)
		}
	}

	return h.renderFactionChoice(c, sender.ID)
}

func (h *OnboardingHandler) renderFactionChoice(c telebot.Context, senderID int64) error {
	selector := &telebot.ReplyMarkup{}
	btnVanguard := selector.Data("🛡️ Metal Vanguard", "join_faction", "steel_vanguard", fmt.Sprintf("%d", senderID))
	btnNomads := selector.Data("⚙️ Rust Nomads", "join_faction", "rust_nomads", fmt.Sprintf("%d", senderID))

	selector.Inline(
		selector.Row(btnVanguard),
		selector.Row(btnNomads),
	)

	welcomeText := "━━━━━━━━━━━━━━━━━━━━━━\n" +
		"⚠️ SYSTEM INTRUSION DETECTED\n" +
		"━━━━━━━━━━━━━━━━━━━━━━\n" +
		"WARNING: Faction registration required. Deploy your core systems:\n\n" +
		"🛡️ [Metal Vanguard]\n" +
		"High-Tech remnant order. Focuses on electricity conservation.\n" +
		"Starting Bonus: +50.0 Electricity Cells\n\n" +
		"⚙️ [Rust Nomads]\n" +
		"Scrappy survival coalition. Focuses on resource collection.\n" +
		"Starting Bonus: +150.0 Scrap\n" +
		"━━━━━━━━━━━━━━━━━━━━━━"

	return c.Send(welcomeText, selector)
}

// nameChangeCostCrystal and nameChangeCostDollars are deliberately steep -
// SpaceHunt's /name command is a rare, deliberate vanity purchase, not
// something players do casually.
const nameChangeCostCrystal = 1000.0
const nameChangeCostDollars = 500.0

// HandleRenameOutpost implements SpaceHunt's "Change your username"
// feature. The new name applies everywhere the player is identified:
// battle reports, the Global Ranking board, World Boss/Rebellion
// leaderboards, and raid targeting.
func (h *OnboardingHandler) HandleRenameOutpost(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	ctx := context.Background()
	newName := strings.TrimSpace(c.Message().Payload)

	if newName == "" {
		return c.Send(fmt.Sprintf(
			"✏️ RENAME OUTPOST\n\nUsage: /name [new name]\n\n💰 Cost: %.0f Crystal + $%.0f\n📏 3-20 characters, letters/numbers/spaces/hyphens only.\n\n⚠️ This changes your public display name everywhere - battle reports, rankings, and leaderboards.",
			nameChangeCostCrystal, nameChangeCostDollars,
		))
	}

	if len(newName) < 3 || len(newName) > 20 {
		return c.Send("❌ Invalid Length: Name must be 3-20 characters.")
	}

	validName := regexp.MustCompile(`^[a-zA-Z0-9 \-]+$`)
	if !validName.MatchString(newName) {
		return c.Send("❌ Invalid Characters: Only letters, numbers, spaces, and hyphens are allowed.")
	}

	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	var existing string
	err = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE LOWER(name) = LOWER($1) AND id != $2", newName, campID).Scan(&existing)
	if err == nil {
		return c.Send("❌ Name Taken: Another survivor already claims that name.")
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Send("⚠️ Rename transaction failed.")
	}
	defer tx.Rollback()

	var crystal, dollars float64
	_ = tx.QueryRowContext(ctx, "SELECT crystal, dollars FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&crystal, &dollars)

	if crystal < nameChangeCostCrystal || dollars < nameChangeCostDollars {
		return c.Send(fmt.Sprintf("❌ Insufficient Funds: Need %.0f Crystal + $%.0f. You have %.0f Crystal + $%.0f.", nameChangeCostCrystal, nameChangeCostDollars, crystal, dollars))
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET crystal = crystal - $1, dollars = dollars - $2 WHERE encampment_id = $3", nameChangeCostCrystal, nameChangeCostDollars, campID)
	_, err = tx.ExecContext(ctx, "UPDATE encampments SET name = $1 WHERE id = $2", newName, campID)
	if err != nil {
		return c.Send("⚠️ Error writing new outpost name.")
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing outpost rename: %v", err)
		return c.Send("⚠️ Error saving changes.")
	}

	return c.Send(fmt.Sprintf("✅ OUTPOST RENAMED: You are now known as \"%s\" across the Wasteland.", newName))
}

// HandleGuide (/guide) re-sends the game briefing and getting-started
// quickstart shown on first spawn, so a player can pull it back up any
// time — e.g. if they skipped past it, or want to re-check the referral
// reward numbers or agent-trial reminder.
func (h *OnboardingHandler) HandleGuide(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)
	text := gameDescriptionText + "\n\n" + gettingStartedGuideText
	return c.Send(text, keyboards.MainNavigation())
}

func (h *OnboardingHandler) HandleHelp(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	helpManual := "━━━━━━━━━━━━━━━━━━━━━━\n" +
		"📖 SURVIVAL TRAINING MANUAL & TUTORIAL\n" +
		"━━━━━━━━━━━━━━━━━━━━━━\n" +
		"Welcome, survivor. This guide explains the core operational loops:\n\n" +
		"⛺ [⛺ Outpost Camp Menu]\n" +
		"• Structural Upgrades: Spend Scrap to level up Tent, Scrap Heap, and Generator.\n" +
		"• Automation Agent: Gated module. Automatically builds facilities and gathers Scrap.\n\n" +
		"⚔️ [⚔️ Tactical Combat Menu]\n" +
		"• Scan Targets: Locate neighboring outposts. Launch raids using your Soldiers/Enforcers.\n" +
		"• Wasteland Radio: Real-time broadcast news detailing sector collapses, storms, and wars.\n\n" +
		"🏦 [🏦 System Economy Menu]\n" +
		"• Financial Vault: Deposit Scrap to earn interest or secure emergency credit lines.\n" +
		"• Clan Alliances: Establish or join forces (capped at 15 members). Trigger alliance wars.\n" +
		"• Heavy Workshop: Spend heavy resources (Metal, Crystal, Hydrogen) to assemble Fusion Tanks.\n\n" +
		"📦 /warehouse — full breakdown of all nine resources at once.\n" +
		"🗺️ /guide — resend the game briefing + first-10-minutes quickstart.\n" +
		"👥 /refer — get your personal invite link and referral rewards.\n\n" +
		"💡 SYSTEM TIP: Tapping '⬅️ Back to HQ' at any time restores the mother navigation keyboard.\n" +
		"━━━━━━━━━━━━━━━━━━━━━━"

	return c.Send(helpManual, keyboards.MainNavigation())
}

func (h *OnboardingHandler) HandleFactionCallback(c telebot.Context) error {
	ctx := context.Background()

	faction := c.Args()[0]
	telegramIDStr := c.Args()[1]

	telegramID, err := strconv.ParseInt(telegramIDStr, 10, 64)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error parsing registration payload."})
	}

	sender := c.Sender()

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Database transaction failure."})
	}
	defer tx.Rollback()

	// Every player gets a referral code the moment their account is
	// created (or upgraded from 'onboarding' to 'active'), instead of
	// waiting for them to first run /refer. COALESCE on conflict means
	// an existing code (e.g. set by an earlier session) is never
	// clobbered.
	newReferralCode := generateReferralCode(telegramID)
	insertUser := `
		INSERT INTO users (telegram_id, username, first_name, state, faction, referral_code) 
		VALUES ($1, $2, $3, 'active', $4, $5)
		ON CONFLICT (telegram_id) 
		DO UPDATE SET faction = $4, state = 'active', referral_code = COALESCE(users.referral_code, $5)`
	_, err = tx.ExecContext(ctx, insertUser, telegramID, sender.Username, sender.FirstName, faction, newReferralCode)
	if err != nil {
		log.Printf("Failed registering faction user: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Database writing registration error."})
	}

	continents := []string{"Africa", "Europe", "Asia", "Americas"}
	spawnedContinent := continents[sender.ID%4]

	var coordID string
	var x, y int
	var success bool

	// Decouple Seeding Loop: Seeding random source exactly once outside of the coordinate calculation loop
	rSource := rand.NewSource(time.Now().UnixNano() + sender.ID)
	rGen := rand.New(rSource)

	for attempt := 0; attempt < 15; attempt++ {
		// Style Refactor (QF1003): Substituted sequential if-else logic with clean Go switch matching
		switch spawnedContinent {
		case "Africa":
			x = rGen.Intn(991) + 10 // [10, 1000]
			y = rGen.Intn(991) + 10 // [10, 1000]
		case "Europe":
			x = -(rGen.Intn(991) + 10) // [-1000, -10]
			y = rGen.Intn(991) + 10    // [10, 1000]
		case "Asia":
			x = rGen.Intn(991) + 10    // [10, 1000]
			y = -(rGen.Intn(991) + 10) // [-1000, -10]
		default: // Americas
			x = -(rGen.Intn(991) + 10) // [-1000, -10]
			y = -(rGen.Intn(991) + 10) // [-1000, -10]
		}

		biome := "wasteland"
		if rGen.Float64() < 0.30 {
			biome = "ruins"
		}

		insertCoord := `
			INSERT INTO coordinates (x, y, biome, danger_level, region, terrain) 
			VALUES ($1, $2, $3, 1, $4, $3) 
			ON CONFLICT (x, y) DO NOTHING
			RETURNING id`

		err = tx.QueryRowContext(ctx, insertCoord, x, y, biome, spawnedContinent).Scan(&coordID)
		if err == nil {
			success = true
			break
		}
	}

	if !success {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Spawning coordinate allocator failed. Please retry registration."})
	}

	var campID string
	queryCampExists := `SELECT id FROM encampments WHERE user_id = $1`
	err = tx.QueryRowContext(ctx, queryCampExists, telegramID).Scan(&campID)
	isNewCamp := errors.Is(err, sql.ErrNoRows)
	var startingScrap, startingEnergy float64
	if isNewCamp {
		campName := fmt.Sprintf("Outpost-%d", telegramID%1000)
		insertCamp := `
			INSERT INTO encampments (user_id, name, coordinate_id, level) 
			VALUES ($1, $2, $3, 1) 
			RETURNING id`
		err = tx.QueryRowContext(ctx, insertCamp, telegramID, campName, coordID).Scan(&campID)
		if err != nil {
			log.Printf("Failed creating encampment: %v", err)
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Camp allocation error."})
		}

		startingScrap = 1000.0
		startingEnergy = 250.0
		if faction == "steel_vanguard" {
			startingEnergy += 500.0
		} else {
			startingScrap += 1500.0
		}

		// Welcome Pack: every brand-new survivor starts with a stake in
		// ALL nine resources, not just the three the old flow granted
		// (Scrap/Rations/Electricity, with everything else defaulting to
		// zero). Metal/Hydrogen/Dollars/Neuro give a real head start on
		// early buildings and trades; Crystal and Ether stay deliberately
		// small since they're the game's scarce/premium resources.
		const (
			startingNeuro    = 50.0
			startingMetal    = 200.0
			startingCrystal  = 20.0
			startingHydrogen = 40.0
			startingDollars  = 300.0
			startingEther    = 5.0
		)

		insertRes := `
			INSERT INTO resources (encampment_id, scrap, rations, electricity, neuro_cores, metal, crystal, hydrogen, dollars, ether) 
			VALUES ($1, $2, 50.00, $3, $4, $5, $6, $7, $8, $9)`
		_, err = tx.ExecContext(ctx, insertRes, campID, startingScrap, startingEnergy,
			startingNeuro, startingMetal, startingCrystal, startingHydrogen, startingDollars, startingEther)
		if err != nil {
			log.Printf("Failed creating resources: %v", err)
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Resource allocation error."})
		}

		_, _ = tx.ExecContext(ctx, "INSERT INTO workshop_inventory (encampment_id) VALUES ($1) ON CONFLICT DO NOTHING", campID)

		// 7-day Cognitive Agent (Premium) trial for every new survivor.
		// This only ever runs inside the isNewCamp guard, so it can't be
		// replayed to keep extending someone's trial.
		_, _ = tx.ExecContext(ctx, "UPDATE users SET premium_until = NOW() + INTERVAL '7 days' WHERE telegram_id = $1", telegramID)
	}

	if err := tx.Commit(); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Ledger completion error."})
	}

	// Referral reward: if this new survivor arrived via a referral code,
	// grant both them and their referrer a resource bonus. Gated on
	// isNewCamp so this can only ever fire once per player — previously
	// this ran on every HandleFactionCallback invocation, letting anyone
	// replay the still-live "join faction" button to farm free resources
	// indefinitely.
	var referrerID sql.NullInt64
	_ = h.DB.QueryRowContext(ctx, "SELECT referred_by FROM users WHERE telegram_id = $1", telegramID).Scan(&referrerID)
	if isNewCamp && referrerID.Valid {
		const refMetal, refCrystal, refNeuro = 50000.0, 500.0, 50000.0

		var curMetal, curCrystal, curNeuro float64
		_ = h.DB.QueryRowContext(ctx, "SELECT metal, crystal, neuro_cores FROM resources WHERE encampment_id = $1", campID).Scan(&curMetal, &curCrystal, &curNeuro)
		myCap := storagecap.CapFor(ctx, h.DB, campID)
		newMetal, _ := storagecap.Clamp(curMetal, refMetal, myCap)
		newCrystal, _ := storagecap.Clamp(curCrystal, refCrystal, myCap)
		newNeuro, _ := storagecap.Clamp(curNeuro, refNeuro, myCap)
		_, _ = h.DB.ExecContext(ctx, "UPDATE resources SET metal = $1, crystal = $2, neuro_cores = $3 WHERE encampment_id = $4", newMetal, newCrystal, newNeuro, campID)

		var referrerCampID string
		if refErr := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", referrerID.Int64).Scan(&referrerCampID); refErr == nil {
			var refCurMetal, refCurCrystal, refCurNeuro float64
			_ = h.DB.QueryRowContext(ctx, "SELECT metal, crystal, neuro_cores FROM resources WHERE encampment_id = $1", referrerCampID).Scan(&refCurMetal, &refCurCrystal, &refCurNeuro)
			referrerCap := storagecap.CapFor(ctx, h.DB, referrerCampID)
			refNewMetal, _ := storagecap.Clamp(refCurMetal, refMetal, referrerCap)
			refNewCrystal, _ := storagecap.Clamp(refCurCrystal, refCrystal, referrerCap)
			refNewNeuro, _ := storagecap.Clamp(refCurNeuro, refNeuro, referrerCap)
			_, _ = h.DB.ExecContext(ctx, "UPDATE resources SET metal = $1, crystal = $2, neuro_cores = $3 WHERE encampment_id = $4", refNewMetal, refNewCrystal, refNewNeuro, referrerCampID)
			_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", referrerID.Int64,
				fmt.Sprintf("🎁 %s: %s joined using your code! You both received %s Metal, %s 🔮 Crystal, %s Neuro Cores.",
					htmlBold("REFERRAL BONUS"), htmlCode(htmlEscape(sender.FirstName)), htmlCode("50,000"), htmlCode("500"), htmlCode("50,000")))

			// Milestone tiers: grant a one-time bonus the first time the
			// referrer's total referral count crosses a threshold.
			var referralCount, tierClaimed int
			_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE referred_by = $1", referrerID.Int64).Scan(&referralCount)
			_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(referral_tier_claimed, 0) FROM users WHERE telegram_id = $1", referrerID.Int64).Scan(&tierClaimed)

			for _, m := range referralMilestones {
				if referralCount >= m.Count && tierClaimed < m.Count {
					var mCurMetal, mCurCrystal, mCurNeuro float64
					_ = h.DB.QueryRowContext(ctx, "SELECT metal, crystal, neuro_cores FROM resources WHERE encampment_id = $1", referrerCampID).Scan(&mCurMetal, &mCurCrystal, &mCurNeuro)
					mNewMetal, _ := storagecap.Clamp(mCurMetal, m.Metal, referrerCap)
					mNewCrystal, _ := storagecap.Clamp(mCurCrystal, m.Crystal, referrerCap)
					mNewNeuro, _ := storagecap.Clamp(mCurNeuro, m.Neuro, referrerCap)
					_, _ = h.DB.ExecContext(ctx, "UPDATE resources SET metal = $1, crystal = $2, neuro_cores = $3 WHERE encampment_id = $4", mNewMetal, mNewCrystal, mNewNeuro, referrerCampID)
					_, _ = h.DB.ExecContext(ctx, "UPDATE users SET referral_tier_claimed = $1 WHERE telegram_id = $2", m.Count, referrerID.Int64)
					_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", referrerID.Int64,
						fmt.Sprintf("🏆 %s: %s friends recruited! Bonus: %s Metal, %s 🔮 Crystal, %s Neuro Cores.",
							htmlBold("REFERRAL MILESTONE"), htmlCode(fmt.Sprintf("%d", m.Count)), htmlCode(fmt.Sprintf("%.0f", m.Metal)), htmlCode(fmt.Sprintf("%.0f", m.Crystal)), htmlCode(fmt.Sprintf("%.0f", m.Neuro))))
					tierClaimed = m.Count
				}
			}
		}
	}

	if isNewCamp {
		_ = c.Respond(&telebot.CallbackResponse{Text: "🛰️ Faction system deployed! Welcome survivor."})

		welcome := fmt.Sprintf(
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"🛰️ COGNITIVE CORE BOOTED\n"+
				"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"Welcome to the wastes, Commander %s.\n"+
				"Your terminal is now integrated into [%s].\n"+
				"Your base has successfully spawned in territory: [%s]\n"+
				"📍 Location Coordinates: [X: %d, Y: %d]\n\n"+
				"%s\n\n"+
				"🎁 WELCOME PACK — DEPLOYED TO YOUR WAREHOUSE\n"+
				"⚙️ Scrap: %.0f\n"+
				"🥫 Rations: 50\n"+
				"⚡ Electricity Cells: %.0f\n"+
				"🧠 Neuro Cores: 50\n"+
				"🔩 Metal: 200\n"+
				"🔮 Crystal: 20\n"+
				"🎈 Hydrogen: 40\n"+
				"💵 Dollars: 300\n"+
				"✨ Ether: 5\n\n"+
				"🧠 7-DAY COGNITIVE AGENT TRIAL — ACTIVATED\n"+
				"Your Cognitive Agent (normally a Premium-only module) is unlocked "+
				"free for the next 7 days. Set a mode in /agent and it keeps working "+
				"your outpost even while you're offline — no charge, no card, nothing "+
				"to cancel.\n\n"+
				"%s",
			sender.FirstName, formatFactionLabel(faction), spawnedContinent, x, y,
			gameDescriptionText, startingScrap, startingEnergy, gettingStartedGuideText,
		)

		selector := &telebot.ReplyMarkup{}
		btnWarehouse := selector.Data("📦 Warehouse Stocks", "view_warehouse")
		btnAgent := selector.Data("🧠 Cognitive Agent", "open_agent")
		btnRefer := selector.Data("👥 Refer a Friend", "open_refer")
		btnManual := selector.Data("📖 Survival Manual", "view_manual")
		selector.Inline(
			selector.Row(btnWarehouse, btnAgent),
			selector.Row(btnRefer, btnManual),
		)

		return sendPanelWithNav(c, navCaptionMain, keyboards.MainNavigation(), welcome, selector)
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🛰️ Faction already active."})
	return c.Send(fmt.Sprintf(
		"Your faction is already set to %s, Commander %s. Your Welcome Pack and Cognitive Agent trial were granted "+
			"when your outpost first spawned and can't be re-issued. Use /guide any time for a refresher on getting started.",
		formatFactionLabel(faction), sender.FirstName,
	), keyboards.MainNavigation())
}

// gameDescriptionText is The Vagabond's lore/premise briefing, shown to
// every new survivor on their first spawn and available any time via
// /guide.
const gameDescriptionText = "🌍 THE VAGABOND — SURVIVOR'S BRIEFING\n" +
	"The old world ended in fire and static. What's left is the Wastes: a " +
	"fractured, tick-driven frontier where every outpost, raid, market trade, " +
	"and alliance plays out in real time — even while you're offline. You lead " +
	"one encampment of survivors, scavenging Scrap, Rations, and Electricity " +
	"just to stay alive, then racing up the tech tree toward Metal, Crystal, " +
	"Hydrogen, and the rarest resource of all, Ether. Raiders, rogue drone " +
	"nests, and rival factions won't wait for you to catch up — so build, " +
	"trade, ally, and fight, before the Wastes do it for you."

// gettingStartedGuideText is the step-by-step quickstart, shared between the
// first-spawn welcome message and the standalone /guide command so a player
// can always call it back up later.
const gettingStartedGuideText = "🗺️ GETTING STARTED — YOUR FIRST 10 MINUTES\n" +
	"1️⃣ Tap 📦 Warehouse Stocks below to see everything in your Welcome Pack.\n" +
	"2️⃣ Open the ⛺ Outpost Camp Menu and spend Scrap upgrading your Tent and Generator first.\n" +
	"3️⃣ Boot your 🧠 Cognitive Agent (free for 7 days) and set it to Collector mode so it keeps gathering while you're away.\n" +
	"4️⃣ Once you've got a few Soldiers, open ⚔️ Tactical Combat to scan for nearby targets.\n" +
	"5️⃣ Run /refer to get your personal invite link — 50,000 Metal + 500 🔮 Crystal + 50,000 Neuro Cores per friend who joins, plus bigger milestone bonuses at 5/10/25 referrals.\n" +
	"6️⃣ Whenever you need a refresher on any menu, /help has the full Survival Training Manual, and /guide brings this quickstart back up.\n" +
	"━━━━━━━━━━━━━━━━━━━━━━"

func formatFactionLabel(f string) string {
	if f == "steel_vanguard" {
		return "🛡️ Metal Vanguard"
	}
	return "⚙️ Rust Nomads"
}
