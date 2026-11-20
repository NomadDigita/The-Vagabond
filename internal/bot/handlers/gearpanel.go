package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/gear"
	"gopkg.in/telebot.v3"
)

type GearHandler struct {
	DB *sql.DB
}

func NewGearHandler(db *sql.DB) *GearHandler {
	return &GearHandler{DB: db}
}

func (h *GearHandler) HandleGearPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	var campID string
	if err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID); err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.MainNavigation())
	}

	owned := make(map[string]int)
	equipped := make(map[string]bool)
	rows, err := h.DB.QueryContext(ctx, "SELECT gear_key, quantity, equipped FROM player_gear WHERE encampment_id = $1", campID)
	if err == nil {
		for rows.Next() {
			var key string
			var qty int
			var eq bool
			if scanErr := rows.Scan(&key, &qty, &eq); scanErr == nil {
				owned[key] = qty
				if eq {
					equipped[key] = true
				}
			}
		}
		rows.Close()
	}

	panelText := "⚙️ " + htmlBold("GEAR & CRAFTING") + "\n" + divider + "\n\n"
	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	panelText += htmlBold("🎯 EQUIPPED LOADOUT") + "\n"
	var totalBonus float64
	for _, slot := range []gear.Slot{gear.SlotWeapon, gear.SlotArmor, gear.SlotUtility} {
		found := false
		for _, def := range gear.Catalog {
			if def.Slot != slot || !equipped[def.Key] {
				continue
			}
			panelText += fmt.Sprintf("%s %s (%s) - +%.0f%% resource generation\n", def.Icon, htmlEscape(def.Name), string(def.Rarity), def.ResourceBonusPct)
			totalBonus += def.ResourceBonusPct
			found = true
			break
		}
		if !found {
			panelText += fmt.Sprintf("— %s: empty\n", capitalizeWord(string(slot)))
		}
	}
	panelText += fmt.Sprintf("\n%s Total: %s\n\n", htmlItalic("Combined bonus"), htmlCode(fmt.Sprintf("+%.0f%%", totalBonus)))

	panelText += htmlBold("🎒 INVENTORY") + "\n"
	anyOwned := false
	for _, def := range gear.Catalog {
		qty, has := owned[def.Key]
		if !has || qty <= 0 {
			continue
		}
		anyOwned = true
		status := ""
		if equipped[def.Key] {
			status = " ✅ equipped"
		}
		panelText += fmt.Sprintf("%s %s x%d (%s)%s\n", def.Icon, htmlEscape(def.Name), qty, string(def.Rarity), status)
		if !equipped[def.Key] {
			buttons = append(buttons, selector.Row(selector.Data(fmt.Sprintf("✅ Equip %s", def.Name), "gear_equip", def.Key)))
		}
	}
	if !anyOwned {
		panelText += htmlItalic("Nothing crafted or found yet.") + "\n"
	}
	panelText += "\n"

	panelText += htmlBold("🔨 CRAFT (Common/Rare only - Epic/Legendary drop from World Bosses and Relic Convoys)") + "\n"
	for _, def := range gear.Catalog {
		if !def.Craftable {
			continue
		}
		costParts := ""
		for resource, amount := range def.CraftCost {
			if costParts != "" {
				costParts += ", "
			}
			costParts += fmt.Sprintf("%.0f %s", amount, capitalizeWord(resource))
		}
		afford := gear.AffordCraft(ctx, h.DB, campID, def)
		affordLabel := "✅"
		if !afford {
			affordLabel = "❌"
		}
		panelText += fmt.Sprintf("%s %s (%s) - %s, %.0fm %s\n", def.Icon, htmlEscape(def.Name), string(def.Rarity), costParts, def.CraftMinutes, affordLabel)
		if afford {
			buttons = append(buttons, selector.Row(selector.Data(fmt.Sprintf("🔨 Craft %s", def.Name), "gear_craft", def.Key)))
		}
	}

	var pendingCount int
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM crafting_queue WHERE encampment_id = $1 AND is_completed = FALSE", campID).Scan(&pendingCount)
	if pendingCount > 0 {
		panelText += fmt.Sprintf("\n⏳ %d item(s) currently crafting.\n", pendingCount)
	}

	panelText += "\n" + divider
	selector.Inline(buttons...)

	return sendPanelWithNavHTML(c, navCaptionEconomy, keyboards.EconomyNavigation(), panelText, selector)
}

func (h *GearHandler) HandleGearCraftCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 1 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid craft request."})
	}
	var campID string
	if err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	msg, err := gear.StartCraft(ctx, h.DB, campID, c.Args()[0])
	if err != nil {
		_ = c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: msg})
		return h.HandleGearPanel(c)
	}
	_ = c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: msg})
	return h.HandleGearPanel(c)
}

func (h *GearHandler) HandleGearEquipCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 1 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid equip request."})
	}
	var campID string
	if err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	if err := gear.Equip(ctx, h.DB, campID, c.Args()[0]); err != nil {
		return c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: "❌ Couldn't equip that item."})
	}
	_ = c.Respond(&telebot.CallbackResponse{Text: "✅ Equipped!"})
	return h.HandleGearPanel(c)
}
