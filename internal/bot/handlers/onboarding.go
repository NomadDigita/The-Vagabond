package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
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

func generateReferralCode(telegramID int64) string {
	return "REF" + strings.ToUpper(strconv.FormatInt(telegramID, 36))
}

var referralMilestones = []struct {
	Count                 int
	Metal, Crystal, Neuro float64
}{
	{5, 25000, 250, 25000},
	{10, 75000, 750, 75000},
	{25, 250000, 2500, 250000},
}

func topUpAllResources(ctx context.Context, db *sql.DB, campID string, amount float64) error {
	var scrap, rations, electricity, neuro, metal, crystal, hydrogen, dollars, ether float64
	err := db.QueryRowContext(ctx, "SELECT scrap, rations, electricity, neuro_cores, metal, crystal, hydrogen, dollars, ether FROM resources WHERE encampment_id = $1", campID).
		Scan(&scrap, &rations, &electricity, &neuro, &metal, &crystal, &hydrogen, &dollars, &ether)
	if err != nil {
		return err
	}
	capAmt := storagecap.CapFor(ctx, db, campID)
	newScrap, _ := storagecap.Clamp(scrap, amount, capAmt)
	newRations, _ := storagecap.Clamp(rations, amount, capAmt)
	newElectricity, _ := storagecap.Clamp(electricity, amount, capAmt)
	newNeuro, _ := storagecap.Clamp(neuro, amount, capAmt)
	newMetal, _ := storagecap.Clamp(metal, amount, capAmt)
	newCrystal, _ := storagecap.Clamp(crystal, amount, capAmt)
	newHydrogen, _ := storagecap.Clamp(hydrogen, amount, capAmt)
	newDollars, _ := storagecap.Clamp(dollars, amount, capAmt)
	newEther, _ := storagecap.Clamp(ether, amount, capAmt)
	_, err = db.ExecContext(ctx, `
		UPDATE resources SET scrap = $1, rations = $2, electricity = $3, neuro_cores = $4,
			metal = $5, crystal = $6, hydrogen = $7, dollars = $8, ether = $9
		WHERE encampment_id = $10`,
		newScrap, newRations, newElectricity, newNeuro, newMetal, newCrystal, newHydrogen, newDollars, newEther, campID)
	return err
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

		if user.State == "naming" {
			return sendNamingPrompt(c, user.FirstName)
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
				"⛺ Encampment: %s\n"+
				"📍 Settled near %s\n\n"+
				"CURRENT RESOURCE BALANCES:\n"+
				"⚙️ Scrap: %.1f\n"+
				"🥫 Rations: %.1f\n"+
				"⚡ Electricity Cells: %.1f\n"+
				"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"Use the command manual below to learn terminal shortcuts.",
			user.FirstName, systemState, activeMiners, ownedMiners, ownedMiners-activeMiners, outboundCount,
			formatFactionLabel(user.Faction), htmlEscape(camp.Name), locationDescriptor(myX, myY, region), res.Scrap, res.Rations, res.Electricity,
		)

		selector := &telebot.ReplyMarkup{}
		btnWarehouse := selector.Data("📦 Warehouse Stocks", "view_warehouse")
		btnManual := selector.Data("📖 Survival Manual", "view_manual")

		selector.Inline(
			selector.Row(btnWarehouse, btnManual),
		)

		return sendPanelWithNavHTML(c, navCaptionMain, keyboards.MainNavigation(), dashboard, selector)
	}

	if !errors.Is(err, sql.ErrNoRows) {
		log.Printf("Database check execution failure: %v", err)
		return c.Send("⚠️ Database reading failure.", keyboards.MainNavigation())
	}

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

	welcomeText := htmlBold("⚠️ SYSTEM INTRUSION DETECTED") + "\n" + divider +
		"\nWARNING: Faction registration required. Deploy your core systems:\n\n" +
		htmlBold("🛡️ Metal Vanguard") + "\n" +
		"High-Tech remnant order. Focuses on electricity conservation.\n" +
		"Starting Bonus: +" + htmlCode("500") + " Electricity Cells\n\n" +
		htmlBold("⚙️ Rust Nomads") + "\n" +
		"Scrappy survival coalition. Focuses on resource collection.\n" +
		"Starting Bonus: +" + htmlCode("1,500") + " Scrap\n" +
		divider

	return c.Send(welcomeText, telebot.ModeHTML, selector)
}

const nameChangeCostCrystal = 1000.0
const nameChangeCostDollars = 500.0

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
		), keyboards.MainNavigation())
	}

	if len(newName) < 3 || len(newName) > 20 {
		return c.Send("❌ Invalid Length: Name must be 3-20 characters.", keyboards.MainNavigation())
	}

	validName := regexp.MustCompile(`^[a-zA-Z0-9 \-]+$`)
	if !validName.MatchString(newName) {
		return c.Send("❌ Invalid Characters: Only letters, numbers, spaces, and hyphens are allowed.", keyboards.MainNavigation())
	}

	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.MainNavigation())
	}

	var existing string
	err = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE LOWER(name) = LOWER($1) AND id != $2", newName, campID).Scan(&existing)
	if err == nil {
		return c.Send("❌ Name Taken: Another survivor already claims that name.", keyboards.MainNavigation())
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Send("⚠️ Rename transaction failed.", keyboards.MainNavigation())
	}
	defer tx.Rollback()

	var crystal, dollars float64
	_ = tx.QueryRowContext(ctx, "SELECT crystal, dollars FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&crystal, &dollars)

	if crystal < nameChangeCostCrystal || dollars < nameChangeCostDollars {
		return c.Send(fmt.Sprintf("❌ Insufficient Funds: Need %.0f Crystal + $%.0f. You have %.0f Crystal + $%.0f.", nameChangeCostCrystal, nameChangeCostDollars, crystal, dollars), keyboards.MainNavigation())
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET crystal = crystal - $1, dollars = dollars - $2 WHERE encampment_id = $3", nameChangeCostCrystal, nameChangeCostDollars, campID)
	_, err = tx.ExecContext(ctx, "UPDATE encampments SET name = $1 WHERE id = $2", newName, campID)
	if err != nil {
		return c.Send("⚠️ Error writing new outpost name.", keyboards.MainNavigation())
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing outpost rename: %v", err)
		return c.Send("⚠️ Error saving changes.", keyboards.MainNavigation())
	}

	return c.Send(fmt.Sprintf("✅ OUTPOST RENAMED: You are now known as \"%s\" across the Wasteland.", newName), keyboards.MainNavigation())
}

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

	coordID, _, _, err := allocateCoordinate(ctx, tx, time.Now().UnixNano()+sender.ID, spawnedContinent)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Spawning coordinate allocator failed. Please retry registration."})
	}

	var campID string
	queryCampExists := `SELECT id FROM encampments WHERE user_id = $1`
	err = tx.QueryRowContext(ctx, queryCampExists, telegramID).Scan(&campID)
	isNewCamp := errors.Is(err, sql.ErrNoRows)
	var startingScrap, startingEnergy float64
	if isNewCamp {
		placeholderName := fmt.Sprintf("Unnamed-Outpost-%d", telegramID)
		insertCamp := `
			INSERT INTO encampments (user_id, name, coordinate_id, level) 
			VALUES ($1, $2, $3, 1) 
			RETURNING id`
		err = tx.QueryRowContext(ctx, insertCamp, telegramID, placeholderName, coordID).Scan(&campID)
		if err != nil {
			log.Printf("Failed creating encampment: %v", err)
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Camp allocation error."})
		}

		const startingBaseline = 25000.0
		startingScrap = startingBaseline
		startingEnergy = startingBaseline
		if faction == "steel_vanguard" {
			startingEnergy += 500.0
		} else {
			startingScrap += 1500.0
		}

		const (
			startingRations  = startingBaseline
			startingNeuro    = startingBaseline
			startingMetal    = startingBaseline
			startingCrystal  = startingBaseline
			startingHydrogen = startingBaseline
			startingDollars  = startingBaseline
			startingEther    = startingBaseline
		)

		insertRes := `
			INSERT INTO resources (encampment_id, scrap, rations, electricity, neuro_cores, metal, crystal, hydrogen, dollars, ether) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
		_, err = tx.ExecContext(ctx, insertRes, campID, startingScrap, startingRations, startingEnergy,
			startingNeuro, startingMetal, startingCrystal, startingHydrogen, startingDollars, startingEther)
		if err != nil {
			log.Printf("Failed creating resources: %v", err)
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Resource allocation error."})
		}

		_, _ = tx.ExecContext(ctx, "INSERT INTO workshop_inventory (encampment_id) VALUES ($1) ON CONFLICT DO NOTHING", campID)

		_, _ = tx.ExecContext(ctx, "UPDATE users SET premium_until = NOW() + INTERVAL '7 days' WHERE telegram_id = $1", telegramID)

		_, _ = tx.ExecContext(ctx, "UPDATE users SET state = 'naming' WHERE telegram_id = $1", telegramID)
	}

	if err := tx.Commit(); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Ledger completion error."})
	}

	var referrerID sql.NullInt64
	_ = h.DB.QueryRowContext(ctx, "SELECT referred_by FROM users WHERE telegram_id = $1", telegramID).Scan(&referrerID)
	if isNewCamp && referrerID.Valid {
		const referralBonusPerResource = 25000.0

		if err := topUpAllResources(ctx, h.DB, campID, referralBonusPerResource); err != nil {
			log.Printf("Failed applying referral bonus to new player %d: %v", telegramID, err)
		}

		var referrerCampID string
		if refErr := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", referrerID.Int64).Scan(&referrerCampID); refErr == nil {
			if err := topUpAllResources(ctx, h.DB, referrerCampID, referralBonusPerResource); err != nil {
				log.Printf("Failed applying referral bonus to referrer %d: %v", referrerID.Int64, err)
			}
			_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", referrerID.Int64,
				fmt.Sprintf("🎁 %s: %s joined using your code! You both received %s of every resource.",
					htmlBold("REFERRAL BONUS"), htmlCode(htmlEscape(sender.FirstName)), htmlCode(fmt.Sprintf("%.0f", referralBonusPerResource))))

			var referralCount, tierClaimed int
			_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE referred_by = $1", referrerID.Int64).Scan(&referralCount)
			_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(referral_tier_claimed, 0) FROM users WHERE telegram_id = $1", referrerID.Int64).Scan(&tierClaimed)
			referrerCap := storagecap.CapFor(ctx, h.DB, referrerCampID)

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
		return sendNamingPrompt(c, sender.FirstName)
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🛰️ Faction already active."})
	return c.Send(fmt.Sprintf(
		"Your faction is already set to %s, Commander %s. Your Welcome Pack and Cognitive Agent trial were granted "+
			"when your outpost first spawned and can't be re-issued. Use /guide any time for a refresher on getting started.",
		formatFactionLabel(faction), sender.FirstName,
	), keyboards.MainNavigation())
}

const gameDescriptionText = "🌍 THE VAGABOND — SURVIVOR'S BRIEFING\n" +
	"The old world ended in fire and static. What's left is the Wastes: a " +
	"fractured, tick-driven frontier where every outpost, raid, market trade, " +
	"and alliance plays out in real time — even while you're offline. You lead " +
	"one encampment of survivors, scavenging Scrap, Rations, and Electricity " +
	"just to stay alive, then racing up the tech tree toward Metal, Crystal, " +
	"Hydrogen, and the rarest resource of all, Ether. Raiders, rogue drone " +
	"nests, and rival factions won't wait for you to catch up — so build, " +
	"trade, ally, and fight, before the Wastes do it for you."

const gettingStartedGuideText = "🗺️ GETTING STARTED — YOUR FIRST 10 MINUTES\n" +
	"1️⃣ Tap 📦 Warehouse Stocks below to see everything in your Welcome Pack.\n" +
	"2️⃣ Open the ⛺ Outpost Camp Menu and spend Scrap upgrading your Tent and Generator first.\n" +
	"3️⃣ Boot your 🧠 Cognitive Agent (free for 7 days) and set it to Collector mode so it keeps gathering while you're away.\n" +
	"4️⃣ Once you've got a few Soldiers, open ⚔️ Tactical Combat to scan for nearby targets.\n" +
	"5️⃣ Run /refer to get your personal invite link — 25,000 of every resource per friend who joins, plus bigger milestone bonuses at 5/10/25 referrals.\n" +
	"6️⃣ Check /quests any time — a one-time 🎯 Getting Started checklist pays out $500 each for your first raid, research upgrade, Clan join, and market purchase, on top of daily/weekly quests and permanent /achievements.\n" +
	"7️⃣ Join or found a Clan with /clan for shared defense and donations, and check the 💱 Market Exchange to buy/sell resources with other survivors.\n" +
	"8️⃣ Whenever you need a refresher on any menu, /help has the full Survival Training Manual, and /guide brings this quickstart back up.\n" +
	"━━━━━━━━━━━━━━━━━━━━━━"

func formatFactionLabel(f string) string {
	if f == "steel_vanguard" {
		return "🛡️ Metal Vanguard"
	}
	return "⚙️ Rust Nomads"
}

var outpostNameRegex = regexp.MustCompile(`^[a-zA-Z0-9 \-]+$`)

func sendNamingPrompt(c telebot.Context, firstName string) error {
	prompt := htmlBold("🛰️ ONE LAST STEP") + "\n" + divider +
		"\nWelcome to the wastes, Commander " + htmlEscape(firstName) + ".\n\n" +
		"Before you see your resources or your new home, name your outpost. " +
		"This is your public callsign everywhere you're identified - battle reports, rankings, and leaderboards.\n\n" +
		"✍️ Reply with a name (3-20 characters, letters/numbers/spaces/hyphens only)."
	return c.Send(prompt, telebot.ModeHTML)
}

func (h *OnboardingHandler) HandleOnboardingPendingInput(c telebot.Context) (handled bool, err error) {
	sender := c.Sender()
	if sender == nil {
		return false, nil
	}

	ctx := context.Background()
	var state string
	var campID string
	dbErr := h.DB.QueryRowContext(ctx, `
		SELECT u.state, e.id FROM users u
		JOIN encampments e ON e.user_id = u.telegram_id
		WHERE u.telegram_id = $1`, sender.ID).Scan(&state, &campID)
	if dbErr != nil || state != "naming" {
		return false, nil
	}

	newName := strings.TrimSpace(c.Text())
	if len(newName) < 3 || len(newName) > 20 {
		return true, c.Send("❌ Invalid Length: Name must be 3-20 characters. Try again.")
	}
	if !outpostNameRegex.MatchString(newName) {
		return true, c.Send("❌ Invalid Characters: Only letters, numbers, spaces, and hyphens are allowed. Try again.")
	}

	var existing string
	if h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE LOWER(name) = LOWER($1) AND id != $2", newName, campID).Scan(&existing) == nil {
		return true, c.Send("❌ Name Taken: Another survivor already claims that name. Try again.")
	}

	if _, err := h.DB.ExecContext(ctx, "UPDATE encampments SET name = $1 WHERE id = $2", newName, campID); err != nil {
		log.Printf("Failed setting first-time outpost name for %d: %v", sender.ID, err)
		return true, c.Send("⚠️ Error saving your outpost name. Try again.")
	}
	if _, err := h.DB.ExecContext(ctx, "UPDATE users SET state = 'active' WHERE telegram_id = $1", sender.ID); err != nil {
		log.Printf("Failed activating account for %d after naming: %v", sender.ID, err)
	}

	return true, h.sendWelcomeComplete(ctx, c, sender.ID, sender.FirstName, campID, newName)
}

func (h *OnboardingHandler) sendWelcomeComplete(ctx context.Context, c telebot.Context, telegramID int64, firstName, campID, campName string) error {
	var faction string
	var scrap, electricity float64
	var region string
	var x, y int
	err := h.DB.QueryRowContext(ctx, `
		SELECT u.faction, r.scrap, r.electricity, co.region, co.x, co.y
		FROM resources r
		JOIN encampments e ON e.id = r.encampment_id
		JOIN coordinates co ON co.id = e.coordinate_id
		JOIN users u ON u.telegram_id = e.user_id
		WHERE r.encampment_id = $1`, campID).Scan(&faction, &scrap, &electricity, &region, &x, &y)
	if err != nil {
		log.Printf("Failed loading welcome-complete details for %d: %v", telegramID, err)
		return c.Send("⚠️ Your outpost is named and ready, but the welcome summary failed to load. Use /start to see your dashboard.")
	}

	welcome := htmlBold("🛰️ COGNITIVE CORE BOOTED") + "\n" + divider + "\n" +
		fmt.Sprintf("Welcome to the wastes, Commander %s.\n", htmlEscape(firstName)) +
		fmt.Sprintf("Faction: %s\n", formatFactionLabel(faction)) +
		fmt.Sprintf("Your outpost, %s, has spawned near %s.\n\n", htmlBold(htmlEscape(campName)), locationDescriptor(x, y, region)) +
		gameDescriptionText + "\n\n" +
		htmlBold("🎁 WELCOME PACK — DEPLOYED TO YOUR WAREHOUSE") + "\n" +
		fmt.Sprintf("⚙️ Scrap: %s\n🥫 Rations: %s\n⚡ Electricity Cells: %s\n🧠 Neuro Cores: %s\n🔩 Metal: %s\n🔮 Crystal: %s\n🎈 Hydrogen: %s\n💵 Dollars: %s\n✨ Ether: %s\n\n",
			htmlCode(fmt.Sprintf("%.0f", scrap)), htmlCode("25,000"), htmlCode(fmt.Sprintf("%.0f", electricity)), htmlCode("25,000"),
			htmlCode("25,000"), htmlCode("25,000"), htmlCode("25,000"), htmlCode("25,000"), htmlCode("25,000")) +
		htmlBold("🧠 7-DAY COGNITIVE AGENT TRIAL — ACTIVATED") + "\n" +
		"Your Cognitive Agent (normally a Premium-only module) is unlocked free for the next 7 days. " +
		"Set a mode in /agent and it keeps working your outpost even while you're offline — no charge, no card, nothing to cancel.\n\n" +
		gettingStartedGuideText

	selector := &telebot.ReplyMarkup{}
	btnWarehouse := selector.Data("📦 Warehouse Stocks", "view_warehouse")
	btnAgent := selector.Data("🧠 Cognitive Agent", "open_agent")
	btnRefer := selector.Data("👥 Refer a Friend", "open_refer")
	btnManual := selector.Data("📖 Survival Manual", "view_manual")
	selector.Inline(
		selector.Row(btnWarehouse, btnAgent),
		selector.Row(btnRefer, btnManual),
	)

	return sendPanelWithNavHTML(c, navCaptionMain, keyboards.MainNavigation(), welcome, selector)
}
