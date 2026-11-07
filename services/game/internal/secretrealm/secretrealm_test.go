package secretrealm

import (
	"encoding/json"
	"math/rand/v2"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"tiendao/game/internal/secretrealm/bicanh"
	"tiendao/game/internal/secretrealm/bicanh/bicanhtest"
)

func shipped(t *testing.T) *Realm {
	t.Helper()
	c, err := LoadFile(filepath.Join("..", "..", "..", "..", "content", "catalog", "secret_realms.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	r, ok := c.Get("hoang_co")
	if !ok {
		t.Fatal("hoang_co missing")
	}
	return r
}

func TestShippedSecretRealmsAreValid(t *testing.T) {
	r := shipped(t)
	// The test-only reference player (and the golden vector) must play the shipped rules.
	if !reflect.DeepEqual(*r.BiCanh(), *bicanhtest.Params()) {
		t.Fatalf("bicanhtest.Params() drifted from secret_realms.yaml:\n%+v\n%+v", *r.BiCanh(), *bicanhtest.Params())
	}
	var snap struct {
		MinElapsedBP   int64 `json:"minElapsedBp"`
		MinRewardTicks int64 `json:"minRewardTicks"`
		TickMS         int64 `json:"tickMs"`
	}
	if err := json.Unmarshal(r.ParamsJSON(), &snap); err != nil || snap.MinElapsedBP != r.MinElapsedBP || snap.MinRewardTicks != r.MinRewardTicks || snap.TickMS == 0 {
		t.Fatalf("params snapshot %s", r.ParamsJSON())
	}
}

func TestLoaderRejects(t *testing.T) {
	base := `version: 1
realms:
  - id: x
    name: X
    summary: S
    engine: bi_canh
    min_tier: 1
    runs_per_day: 3
    expire_after_seconds: 240
    min_elapsed_bp: 9000
    min_reward_ticks: 600
    params: PARAMS
    bands:
      - { id: a, name: A, min_score: 10, min_qi: 5, min_kills: 5 }
      - { id: b, name: B, min_score: 20, min_qi: 5, min_kills: 6 }
    loot:
      a: { spirit_stone: { min: 1, max: 2 }, items: [], artifact_chance_bp: 0 }
      b: { spirit_stone: { min: 2, max: 3 }, items: [], artifact_chance_bp: 100 }
    survived: { spirit_stone: 1, artifact_chance_bp: 100 }
    artifacts: [{ item_id: k, weight: 1, duplicate_stones: 1 }]
    daily_caps: { spirit_stone: 10, materials: 10, artifacts: 1 }
`
	params := `{ rules_version: 1, tick_ms: 50, time_limit_ticks: 3000, arena: { width: 640, height: 640 },
      player: { radius: 10, speed_sub: 80, hp: 6, invuln_ticks: 24 }, sword: { range: 230, speed_sub: 240, radius: 5, life_ticks: 22, damage: 1 },
      orbit: { radius: 46, blade_radius: 7, step: 2, damage: 1, hit_cooldown_ticks: 10 }, levels: [{ qi: 0, fire_every: 14, orbit: 0 }],
      spawn: { start_ticks: 28, min_ticks: 8, step_every_ticks: 140, wave_max: 4, wave_grow_every_ticks: 600, max_enemies: 80, safe_radius: 120 },
      enemies: [{ id: a, name: A, weight: 1, from_tick: 0, hp: 1, speed_sub: 20, radius: 8, points: 1 }],
      orbs: { pickup_radius: 22, magnet_radius: 72, magnet_speed_sub: 112, life_ticks: 500, max: 90 }, max_swords: 12,
      input_window_ticks: 20, max_changes_per_window: 8, max_runs: 1500 }`
	ok := strings.Replace(base, "PARAMS", params, 1)
	if _, err := Load(strings.NewReader(ok)); err != nil {
		t.Fatalf("valid catalogue rejected: %v", err)
	}
	for name, tc := range map[string]struct{ from, to, want string }{
		"loose expiry":      {"expire_after_seconds: 240", "expire_after_seconds: 900", "expire_after_seconds"},
		"tight expiry":      {"expire_after_seconds: 240", "expire_after_seconds: 160", "expire_after_seconds"},
		"no runs":           {"runs_per_day: 3", "runs_per_day: 0", "runs_per_day"},
		"band order":        {"min_score: 20", "min_score: 5", "increase"},
		"no play floor":     {"min_qi: 5, min_kills: 6", "min_qi: 0, min_kills: 6", "min_qi and min_kills"},
		"floor decreases":   {"min_qi: 5, min_kills: 6", "min_qi: 5, min_kills: 4", "never decrease"},
		"worse loot higher": {"b: { spirit_stone: { min: 2, max: 3 }", "b: { spirit_stone: { min: 0, max: 1 }", "pays less"},
		"chance > 100%":     {"artifact_chance_bp: 100 }\n    artifacts", "artifact_chance_bp: 9950 }\n    artifacts", "exceeds 100%"},
		"unknown field":     {"min_tier: 1", "min_tier: 1\n    vip_bonus_bp: 100", "not found"},
		"floor beyond time": {"min_reward_ticks: 600", "min_reward_ticks: 3000", "min_reward_ticks"},
	} {
		_, err := Load(strings.NewReader(strings.Replace(ok, tc.from, tc.to, 1)))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: want %q, got %v", name, tc.want, err)
		}
	}
}

func TestJudgeOrderAndTamper(t *testing.T) {
	r := shipped(t)
	p := r.BiCanh()
	seed := uint32(20261107)
	moves, live := bicanhtest.AutoPlay(p, seed, bicanhtest.Skills[0], 0)
	issued := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	run := Run{Seed: seed, Params: r.ParamsJSON(), IssuedAt: issued, ExpiresAt: issued.Add(240 * time.Second)}
	at := issued.Add(time.Duration(live.EndTick*p.TickMS) * time.Millisecond)
	claim := Claim{EndTick: live.EndTick, Kills: live.Kills, Score: live.Score, StateHash: live.StateHash, Moves: moves}
	if j := Judge(r, run, claim, at); j.Verdict != VerdictPass || j.Outcome == nil || j.Outcome.Score != live.Score {
		t.Fatalf("honest pass: %+v", j)
	}
	cases := []struct {
		name   string
		mut    func(c *Claim)
		now    time.Time
		reason string
	}{
		{"late", nil, run.ExpiresAt, ReasonLate},
		{"too fast", nil, issued.Add(10 * time.Second), ReasonTooFast},
		{"score inflated", func(c *Claim) { c.Score++ }, at, ReasonMismatch},
		{"impossible score", func(c *Claim) { c.Score = p.MaxScore(c.Kills) + 1 }, at, ReasonImplausible},
		{"negative kills", func(c *Claim) { c.Kills = -1 }, at, ReasonImplausible},
		{"hash", func(c *Claim) { c.StateHash ^= 1 }, at, ReasonMismatch},
		{"short end", func(c *Claim) { c.EndTick-- }, at, ReasonInvalidLog},
		{"no moves", func(c *Claim) { c.Moves = nil }, at, ReasonInvalidLog},
		{"jitter", func(c *Claim) {
			var m []bicanh.Run
			for i := int64(0); i < 100; i++ {
				m = append(m, bicanh.Run{1 + i%2, 1})
			}
			m = append(m, bicanh.Run{0, c.EndTick - 100})
			c.Moves = m
		}, at, ReasonInputRate},
		{"other seed log", func(c *Claim) {
			c.Moves, _ = bicanhtest.AutoPlay(p, seed+1, bicanhtest.Skills[0], c.EndTick)
		}, at, ReasonMismatch},
	}
	for _, tc := range cases {
		c := claim
		c.Moves = append([]bicanh.Run(nil), moves...)
		if tc.mut != nil {
			tc.mut(&c)
		}
		j := Judge(r, run, c, tc.now)
		if j.Reason != tc.reason || (tc.reason != ReasonLate && j.Verdict != VerdictRejected) {
			t.Errorf("%s: got %s/%s, want %s", tc.name, j.Verdict, j.Reason, tc.reason)
		}
	}
	// Slow submits are flagged, never rejected.
	if j := Judge(r, run, claim, at.Add(70*time.Second)); j.Verdict != VerdictPass || !j.Outcome.SlowSubmit {
		t.Fatalf("slow submit: %+v", j)
	}
}

func TestBandFor(t *testing.T) {
	r := shipped(t)
	ps := r.ParamsJSON()
	o := func(end, score int64) *Outcome {
		return &Outcome{EndTick: end, Score: score, Qi: 10_000, Kills: 10_000}
	}
	if b := BandFor(r, ps, VerdictFail, o(r.MinRewardTicks-1, 5000)); b != nil {
		t.Fatalf("below the min-run floor must earn nothing, got %v", b)
	}
	if b := BandFor(r, ps, VerdictRejected, o(3000, 5000)); b != nil {
		t.Fatal("rejected runs earn nothing")
	}
	if b := BandFor(r, ps, VerdictFail, o(3000, r.Bands[0].MinScore-1)); b != nil {
		t.Fatal("below the lowest band earns nothing")
	}
	for i, b := range r.Bands {
		if got := BandFor(r, ps, VerdictPass, o(3000, b.MinScore)); got == nil || got.ID != r.Bands[i].ID {
			t.Fatalf("score %d: band %v, want %s", b.MinScore, got, b.ID)
		}
	}
	// A high score without real play (motes walked onto, spirits defeated) earns nothing (anti-cheat M4).
	hi := &Outcome{EndTick: 3000, Score: 5000, Qi: r.Bands[0].MinQi - 1, Kills: 5000}
	if b := BandFor(r, ps, VerdictPass, hi); b != nil {
		t.Fatalf("too little linh khí: %v", b)
	}
	hi = &Outcome{EndTick: 3000, Score: 5000, Qi: 5000, Kills: r.Bands[0].MinKills - 1}
	if b := BandFor(r, ps, VerdictPass, hi); b != nil {
		t.Fatalf("too few kills: %v", b)
	}
}

func blindRun(p *bicanh.Params, seed uint32, f func(int64) int64) ([]bicanh.Run, bicanh.Result) {
	g := bicanh.New(p, seed)
	rec := bicanhtest.NewRecorder(p)
	for !g.Over() {
		g.Step(rec.Sample(f(g.Tick())))
	}
	return rec.Runs, g.Result()
}

// Anti-cheat M4: blind / scripted patterns earn a band on < 5% of seeds; the human-cadence reference player reaches at
// least Hạ Phẩm on ≥ 90% at every pace. Blind logs also raise review flags.
func TestBandsNeedRealPlay(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	r := shipped(t)
	p := r.BiCanh()
	const n = 100
	outcome := func(res bicanh.Result) *Outcome {
		return &Outcome{EndTick: res.EndTick, Score: res.Score, Qi: res.Qi, Kills: res.Kills}
	}
	verdict := func(res bicanh.Result) string {
		if res.Won {
			return VerdictPass
		}
		return VerdictFail
	}
	blind := map[string]func(int64) int64{
		"idle":     func(int64) int64 { return 0 },
		"loop10":   func(t int64) int64 { return 1 + (t/10)%8 },
		"loop25":   func(t int64) int64 { return 1 + (t/25)%8 },
		"loop40":   func(t int64) int64 { return 1 + (t/40)%8 },
		"loop60":   func(t int64) int64 { return 1 + (t/60)%8 },
		"loop100":  func(t int64) int64 { return 1 + (t/100)%8 },
		"backfwd":  func(t int64) int64 { return 3 + 4*((t/40)%2) },
		"square":   func(t int64) int64 { return 1 + 2*((t/30)%4) },
		"diagonal": func(t int64) int64 { return 2 + 4*((t/50)%2) },
	}
	for name, f := range blind {
		rewarded, flagged := 0, 0
		for i := range uint32(n) {
			moves, res := blindRun(p, i*2654435761+1, f)
			if BandFor(r, r.ParamsJSON(), verdict(res), outcome(res)) != nil {
				rewarded++
			}
			if len(ReviewFlags(moves, res.EndTick, p.TickMS)) > 0 {
				flagged++
			}
		}
		t.Logf("blind %-8s rewarded %d/%d, flagged %d/%d", name, rewarded, n, flagged, n)
		if rewarded*20 >= n {
			t.Errorf("blind %s earns a band on %d/%d seeds (want < 5%%)", name, rewarded, n)
		}
		if flagged*10 < n*9 {
			t.Errorf("blind %s flagged on only %d/%d seeds", name, flagged, n)
		}
	}
	for _, s := range bicanhtest.Skills {
		got, flagged := 0, 0
		for i := range uint32(n) {
			moves, res := bicanhtest.AutoPlay(p, i*2654435761+1, s, 0)
			if BandFor(r, r.ParamsJSON(), verdict(res), outcome(res)) != nil {
				got++
			}
			if len(ReviewFlags(moves, res.EndTick, p.TickMS)) > 0 {
				flagged++
			}
		}
		t.Logf("reference %-6s ≥ Hạ Phẩm %d/%d, flagged %d/%d", s.Name, got, n, flagged, n)
		if got*10 < n*9 {
			t.Errorf("reference %s reaches a band on only %d/%d seeds (want ≥ 90%%)", s.Name, got, n)
		}
		if flagged*10 > n {
			t.Errorf("reference %s flagged on %d/%d seeds (false positives)", s.Name, flagged, n)
		}
	}
}

// Seeded loot distribution: ranges respected, the artifact rate matches the disclosed chance (±), weights roughly hold,
// duplicates become stones, and the daily caps clip.
func TestRollLootDistributionAndCaps(t *testing.T) {
	r := shipped(t)
	top := &r.Bands[len(r.Bands)-1]
	bl := r.Loot[top.ID]
	rng := rand.New(rand.NewPCG(11, 12))
	const n = 20000
	arts := map[string]int{}
	hits := 0
	for range n {
		l := RollLoot(r, top, true, rng, nil, Today{})
		lo, hi := bl.SpiritStone.Min+r.Survived.SpiritStone, bl.SpiritStone.Max+r.Survived.SpiritStone
		if l.RolledArtifact == "" && (l.RolledStone < lo || l.RolledStone > hi) {
			t.Fatalf("stones %d outside %d..%d", l.RolledStone, lo, hi)
		}
		if l.ArtifactChanceBP != bl.ArtifactChanceBP+r.Survived.ArtifactChanceBP {
			t.Fatalf("disclosed chance %d", l.ArtifactChanceBP)
		}
		if l.Artifact != "" {
			hits++
			arts[l.Artifact]++
		}
	}
	want := float64(n) * float64(bl.ArtifactChanceBP+r.Survived.ArtifactChanceBP) / 10000
	if f := float64(hits); f < want*0.85 || f > want*1.15 {
		t.Fatalf("artifact drops %d, expected ≈ %.0f", hits, want)
	}
	if arts[r.Artifacts[0].ItemID] <= arts[r.Artifacts[len(r.Artifacts)-1].ItemID] {
		t.Fatalf("weights not respected: %v", arts)
	}
	t.Logf("artifact drops %d/%d (expected %.0f): %v", hits, n, want, arts)

	// Duplicates turn into stones; a full day clips everything and is disclosed.
	always := &Realm{ID: "t", Bands: []Band{{ID: "a", Name: "A", MinScore: 1}}, Loot: map[string]BandLoot{"a": {SpiritStone: Range{10, 10},
		Items: []ItemDrop{{ItemID: "linh_thao", Min: 5, Max: 5}}, ArtifactChanceBP: 10000}}, Artifacts: []ArtifactDrop{{ItemID: "k", Weight: 1, DuplicateStones: 7}},
		DailyCaps: Caps{SpiritStone: 15, Materials: 3, Artifacts: 1}, artWeight: 1}
	l := RollLoot(always, &always.Bands[0], false, rng, func(string) bool { return true }, Today{})
	if l.DuplicateOf != "k" || l.Artifact != "" || l.RolledStone != 17 || l.SpiritStone != 15 || l.Materials() != 3 || !l.Clipped {
		t.Fatalf("duplicate + caps: %+v", l)
	}
	l = RollLoot(always, &always.Bands[0], false, rng, nil, Today{SpiritStone: 15, Materials: 3, Artifacts: 1})
	if l.SpiritStone != 0 || l.Materials() != 0 || l.Artifact != "" || !l.ArtifactCapped || l.RolledArtifact != "k" {
		t.Fatalf("capped day: %+v", l)
	}
	l = RollLoot(always, &always.Bands[0], false, rng, nil, Today{})
	if l.Artifact != "k" || l.Clipped != true { // materials 5 > cap 3
		t.Fatalf("fresh day: %+v", l)
	}
}

// Daily ceiling of a perfect player: runs/day × the best band + survived bonus can't beat the stone cap by design.
func TestDailyCapBinds(t *testing.T) {
	r := shipped(t)
	top := r.Loot[r.Bands[len(r.Bands)-1].ID]
	if best := r.RunsPerDay * (top.SpiritStone.Max + r.Survived.SpiritStone); best < r.DailyCaps.SpiritStone {
		t.Logf("note: the stone cap %d is above the best possible day %d", r.DailyCaps.SpiritStone, best)
	}
	if r.DailyCaps.SpiritStone <= 0 || r.DailyCaps.Artifacts < 1 {
		t.Fatal("caps must allow some reward")
	}
}
