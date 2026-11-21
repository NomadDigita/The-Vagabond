package models

import (
	"time"
)

type User struct {
	TelegramID   int64     `json:"telegram_id"`
	Username     string    `json:"username"`
	FirstName    string    `json:"first_name"`
	State        string    `json:"state"`
	Faction      string    `json:"faction"`
	RegisteredAt time.Time `json:"registered_at"`
	LastActive   time.Time `json:"last_active"`
}

type Coordinate struct {
	ID                string  `json:"id"`
	X                 int     `json:"x"`
	Y                 int     `json:"y"`
	Biome             string  `json:"biome"`
	DangerLevel       int     `json:"danger_level"`
	ScrapMultiplier   float64 `json:"scrap_multiplier"`
	RationsMultiplier float64 `json:"rations_multiplier"`
	EnergyMultiplier  float64 `json:"energy_multiplier"`
}

type Encampment struct {
	ID            string    `json:"id"`
	UserID        int64     `json:"user_id"`
	Name          string    `json:"name"`
	CoordinateID  string    `json:"coordinate_id"`
	Level         int       `json:"level"`
	EstablishedAt time.Time `json:"established_at"`
}

type Resources struct {
	EncampmentID string    `json:"encampment_id"`
	Scrap        float64   `json:"scrap"`
	Rations      float64   `json:"rations"`
	Electricity  float64   `json:"electricity"`
	NeuroCores   float64   `json:"neuro_cores"`
	Metal        float64   `json:"metal"`
	Crystal      float64   `json:"crystal"`
	Hydrogen     float64   `json:"hydrogen"`
	Dollars      float64   `json:"dollars"`
	LastTickedAt time.Time `json:"last_ticked_at"`
}

type Hero struct {
	ID              string `json:"id"`
	EncampmentID    string `json:"encampment_id"`
	Name            string `json:"name"`
	Trait           string `json:"trait"`
	Injuries        string `json:"injuries"`
	BattlesSurvived int    `json:"battles_survived"`
	Superpower      string `json:"superpower"`
}

type WorkshopInventory struct {
	EncampmentID   string `json:"encampment_id"`
	FusionTanks    int    `json:"fusion_tanks"`
	NuclearShields int    `json:"nuclear_shields"`
	Soldiers       int    `json:"soldiers"`
	Drones         int    `json:"drones"`
	Jets           int    `json:"jets"`
	Mechs          int    `json:"mechs"`
	Nukes          int    `json:"nukes"`
	Buggies        int    `json:"buggies"`
	Ships          int    `json:"ships"`
	Haulers        int    `json:"haulers"`
	Tankers        int    `json:"tankers"`
	Rigs           int    `json:"rigs"`
}

type RaidCoopMember struct {
	RaidID              string `json:"raid_id"`
	EncampmentID        string `json:"encampment_id"`
	SoldiersContributed int    `json:"soldiers_contributed"`
	MechsContributed    int    `json:"mechs_contributed"`
}

type ArenaBattle struct {
	ID             string    `json:"id"`
	Bracket        string    `json:"bracket"`
	WinnerUsername string    `json:"winner_username"`
	LoserUsername  string    `json:"loser_username"`
	WinnerLoot     float64   `json:"winner_loot"`
	BattleTime     time.Time `json:"battle_time"`
}
