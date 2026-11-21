package scoring

const ScoreExpr = `(
	(e.level * 1000)
	+ COALESCE((SELECT SUM(level) FROM modules m WHERE m.encampment_id = e.id), 0) * 500
	+ COALESCE((
		SELECT (scrap + rations + electricity + neuro_cores + metal + crystal + hydrogen + (dollars * 2))
		FROM resources r WHERE r.encampment_id = e.id
	), 0) * 0.05
	+ COALESCE((
		SELECT (soldiers * 10) + (drones * 50) + (mechs * 400) + (nukes * 1000)
		     + (buggies * 80) + (ships * 100) + (jets * 150) + (haulers * 120)
		     + (tankers * 130) + (rigs * 140) + (destroyers * 300) + (bombers * 450)
		     + (scouts * 20) + (battlecruisers * 1500)
		     + (liberators * 700) + (wraiths * 350) + (observers * 30) + (guardians * 200)
		     + (piercing_missiles * 800) + (cargo_mk1 * 100) + (cargo_mk2 * 180) + (cargo_mk3 * 280)
		FROM workshop_inventory w WHERE w.encampment_id = e.id
	), 0)
)`

const MilitaryScoreExpr = `COALESCE((
	SELECT (soldiers * 10) + (drones * 50) + (mechs * 400) + (nukes * 1000)
	     + (buggies * 80) + (ships * 100) + (jets * 150) + (haulers * 120)
	     + (tankers * 130) + (rigs * 140) + (destroyers * 300) + (bombers * 450)
	     + (scouts * 20) + (battlecruisers * 1500)
	     + (liberators * 700) + (wraiths * 350) + (observers * 30) + (guardians * 200)
	     + (piercing_missiles * 800) + (cargo_mk1 * 100) + (cargo_mk2 * 180) + (cargo_mk3 * 280)
	FROM workshop_inventory w WHERE w.encampment_id = e.id
), 0)`
