package tick

import "testing"

func TestRoadForcePowerWeightsCombatUnits(t *testing.T) {
	force := roadForce{
		soldiers:       20,
		mechs:          2,
		destroyers:     1,
		bombers:        1,
		battlecruisers: 1,
		deathstars:     1,
		liberators:     1,
		wraiths:        1,
	}
	if got, want := force.power(), 166.0; got != want {
		t.Fatalf("power() = %.0f, want %.0f", got, want)
	}
}

func TestRoadLossesNeverCreatesUnits(t *testing.T) {
	force := roadForce{soldiers: 11, mechs: 3, destroyers: 2, battlecruisers: 1}
	losses, survivors := roadLosses(force, 0.40)
	if survivors.soldiers != 7 || survivors.mechs != 2 || survivors.destroyers != 2 || survivors.battlecruisers != 1 {
		t.Fatalf("unexpected survivors: %#v", survivors)
	}
	if losses[0].Count != 4 || losses[1].Count != 1 || losses[2].Count != 0 || losses[6].Count != 0 {
		t.Fatalf("unexpected losses: %#v", losses)
	}
	if survivors.power() > force.power() {
		t.Fatalf("losses increased force power: before %.0f after %.0f", force.power(), survivors.power())
	}
}
