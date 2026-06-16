package models

import (
	"time"
)

// User represents a Telegram account registered in the system.
type User struct {
	TelegramID   int64     `json:"telegram_id"`
	Username     string    `json:"username"`
	FirstName    string    `json:"first_name"`
	State        string    `json:"state"`   // "onboarding", "active", "dead"
	Faction      string    `json:"faction"` // "steel_vanguard", "rust_nomads"
	RegisteredAt time.Time `json:"registered_at"`
	LastActive   time.Time `json:"last_active"`
}

// Coordinate represents a structural map grid location.
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

// Encampment represents a player's physical home base settlement.
type Encampment struct {
	ID            string    `json:"id"`
	UserID        int64     `json:"user_id"`
	Name          string    `json:"name"`
	CoordinateID  string    `json:"coordinate_id"`
	Level         int       `json:"level"`
	EstablishedAt time.Time `json:"established_at"`
}

// Resources represent the running currency balances of an Encampment.
type Resources struct {
	EncampmentID string    `json:"encampment_id"`
	Scrap        float64   `json:"scrap"`
	Rations      float64   `json:"rations"`
	Energy       float64   `json:"energy"`
	NeuroCores   float64   `json:"neuro_cores"`
	Steel        float64   `json:"steel"`
	Uranium      float64   `json:"uranium"`
	Hydrogen     float64   `json:"hydrogen"`
	Dollars      float64   `json:"dollars"`
	LastTickedAt time.Time `json:"last_ticked_at"`
}

// Hero represents a legendary commander tracking survived encounters, traits, and scars.
type Hero struct {
	ID              string `json:"id"`
	EncampmentID    string `json:"encampment_id"`
	Name            string `json:"name"`
	Trait           string `json:"trait"`    // e.g. "Never Retreat", "Resource Finder"
	Injuries        string `json:"injuries"` // e.g. "Cybernetic Leg", "Scarred Eye"
	BattlesSurvived int    `json:"battles_survived"`
	Superpower      string `json:"superpower"` // e.g. "Energy Shielding", "Scrap Recovery"
}
