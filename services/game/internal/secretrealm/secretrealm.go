// Package secretrealm is the bí cảnh thử luyện (Phase 3.4, cot-truyen §5.5): a daily resource mini game. The server
// issues a seeded run, the client plays it and submits its movement log once, the server replays it
// deterministically (sub-package bicanh) and grants capped loot — linh thạch, materials and pháp bảo — through the
// currency/inventory ledgers in the same transaction.
//
// secretrealm.go holds the catalogue (content/catalog/secret_realms.yaml) and the pure rules (Judge, Band, RollLoot);
// service.go owns the transactions. Nothing here reads Tiên Duyên: runs/day, loot and caps are the same for everyone.
package secretrealm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"

	"tiendao/game/internal/fixedpoint"
	"tiendao/game/internal/secretrealm/bicanh"
)

// EngineBiCanh is the only engine today.
const EngineBiCanh = "bi_canh"

// MaxClaimBytes is the request-body limit of POST /secret-realms/runs/{runId}/submit. The loader checks the longest
// legal log (max_runs × ClaimBytesPerRun) fits.
const (
	MaxClaimBytes      = 32 << 10
	ClaimBytesPerRun   = 12 // "[8,3000],"
	claimOverheadBytes = 512
)

// SlowSubmitSlackMillis: a submit later than game time + this is flagged for review (seed foreknowledge), never rejected.
const SlowSubmitSlackMillis = 60_000

// Range is an inclusive integer range.
type Range struct {
	Min int64 `yaml:"min" json:"min"`
	Max int64 `yaml:"max" json:"max"`
}

// ItemDrop is a non-artifact item drop of a band.
type ItemDrop struct {
	ItemID string `yaml:"item_id" json:"itemId"`
	Min    int64  `yaml:"min" json:"min"`
	Max    int64  `yaml:"max" json:"max"`
}

// BandLoot is the loot of one result band.
type BandLoot struct {
	SpiritStone      Range      `yaml:"spirit_stone" json:"spiritStone"`
	Items            []ItemDrop `yaml:"items" json:"items"`
	ArtifactChanceBP int64      `yaml:"artifact_chance_bp" json:"artifactChanceBp"`
}

// SurvivedBonus is added when the run reached the time limit.
type SurvivedBonus struct {
	SpiritStone      int64 `yaml:"spirit_stone" json:"spiritStone"`
	ArtifactChanceBP int64 `yaml:"artifact_chance_bp" json:"artifactChanceBp"`
}

// Band is a result tier by the replayed run. Every band needs real play, not only a score (anti-cheat M4): enough
// linh khí picked up (you have to walk onto it) and enough spirits defeated.
type Band struct {
	ID       string `yaml:"id" json:"id"`
	Name     string `yaml:"name" json:"name"`
	MinScore int64  `yaml:"min_score" json:"minScore"`
	MinQi    int64  `yaml:"min_qi" json:"minQi"`
	MinKills int64  `yaml:"min_kills" json:"minKills"`
}

// ArtifactDrop is one entry of the pháp bảo table.
type ArtifactDrop struct {
	ItemID          string `yaml:"item_id" json:"itemId"`
	Weight          int64  `yaml:"weight" json:"weight"`
	DuplicateStones int64  `yaml:"duplicate_stones" json:"duplicateStones"`
}

// Caps are the daily reward caps per character (game day of the submit).
type Caps struct {
	SpiritStone int64 `yaml:"spirit_stone" json:"spiritStone"`
	Materials   int64 `yaml:"materials" json:"materials"`
	Artifacts   int64 `yaml:"artifacts" json:"artifacts"`
}

// Realm is one secret realm. Immutable after load.
type Realm struct {
	ID                 string              `yaml:"id" json:"id"`
	Name               string              `yaml:"name" json:"name"`
	Summary            string              `yaml:"summary" json:"summary"`
	Engine             string              `yaml:"engine" json:"engine"`
	MinTier            int                 `yaml:"min_tier" json:"minTier"`
	RunsPerDay         int64               `yaml:"runs_per_day" json:"runsPerDay"`
	ExpireAfterSeconds int64               `yaml:"expire_after_seconds" json:"expireAfterSeconds"`
	MinElapsedBP       int64               `yaml:"min_elapsed_bp" json:"minElapsedBp"`
	MinRewardTicks     int64               `yaml:"min_reward_ticks" json:"minRewardTicks"`
	Params             yaml.Node           `yaml:"params" json:"-"`
	Bands              []Band              `yaml:"bands" json:"bands"`
	Loot               map[string]BandLoot `yaml:"loot" json:"loot"`
	Survived           SurvivedBonus       `yaml:"survived" json:"survived"`
	Artifacts          []ArtifactDrop      `yaml:"artifacts" json:"artifacts"`
	DailyCaps          Caps                `yaml:"daily_caps" json:"dailyCaps"`

	biCanh     *bicanh.Params
	paramsJSON json.RawMessage
	artWeight  int64
}

// BiCanh returns the decoded engine params.
func (r *Realm) BiCanh() *bicanh.Params { return r.biCanh }

// ParamsJSON is the snapshot stored in each run and sent to the client: the engine params plus the run-level rules
// the judge needs (minElapsedBp, minRewardTicks), so a catalogue change never alters a run in flight.
func (r *Realm) ParamsJSON() json.RawMessage { return r.paramsJSON }

// Catalogue holds the secret realms.
type Catalogue struct {
	realms []*Realm
	byID   map[string]*Realm
}

type file struct {
	Version int      `yaml:"version"`
	Realms  []*Realm `yaml:"realms"`
}

var slugRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// LoadFile parses and validates secret_realms.yaml.
func LoadFile(path string) (*Catalogue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// runSnapshot holds the realm-level rules snapshotted next to the engine params in every run.
type runSnapshot struct {
	MinElapsedBP   int64 `json:"minElapsedBp"`
	MinRewardTicks int64 `json:"minRewardTicks"`
}

// Load parses and validates secret realm YAML. Item references are checked in content.LoadAll.
func Load(rd io.Reader) (*Catalogue, error) {
	var raw file
	dec := yaml.NewDecoder(rd)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("secretrealm: decode: %w", err)
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if raw.Version != 1 {
		fail("unsupported version %d", raw.Version)
	}
	if len(raw.Realms) == 0 {
		fail("need at least one realm")
	}
	c := &Catalogue{byID: map[string]*Realm{}}
	for i, r := range raw.Realms {
		if !slugRe.MatchString(r.ID) {
			fail("realm[%d]: invalid id %q", i, r.ID)
			continue
		}
		if _, dup := c.byID[r.ID]; dup {
			fail("realm %q: duplicate id", r.ID)
		}
		if r.Name == "" || r.Summary == "" {
			fail("realm %q: empty name or summary", r.ID)
		}
		if r.MinTier < 1 {
			fail("realm %q: min_tier must be ≥ 1", r.ID)
		}
		if r.RunsPerDay < 1 || r.RunsPerDay > 20 {
			fail("realm %q: runs_per_day must be within 1..20", r.ID)
		}
		if r.MinElapsedBP < 5000 || r.MinElapsedBP > fixedpoint.BPScale {
			fail("realm %q: min_elapsed_bp must be within 5000..10000", r.ID)
		}
		if r.Engine != EngineBiCanh {
			fail("realm %q: unknown engine %q", r.ID, r.Engine)
		} else {
			var p bicanh.Params
			if err := strictParams(&r.Params, &p); err != nil {
				fail("realm %q: params: %v", r.ID, err)
			} else if err := p.Validate(); err != nil {
				fail("realm %q: params: %w", r.ID, err)
			} else {
				if int64(p.MaxRuns)*ClaimBytesPerRun+claimOverheadBytes > MaxClaimBytes {
					fail("realm %q: max_runs × %d bytes exceeds the submit body limit (%d bytes)", r.ID, ClaimBytesPerRun, MaxClaimBytes)
				}
				gameSec := p.TimeLimitTicks * p.TickMS / 1000
				// Tight expiry (seed foreknowledge, anti-cheat M1): the game + 30..120 s of slack.
				if r.ExpireAfterSeconds < gameSec+30 || r.ExpireAfterSeconds > gameSec+120 {
					fail("realm %q: expire_after_seconds must be within time limit (%d s) + 30..120", r.ID, gameSec)
				}
				if r.MinRewardTicks < 1 || r.MinRewardTicks >= p.TimeLimitTicks {
					fail("realm %q: min_reward_ticks must be within 1..time_limit_ticks-1", r.ID)
				}
				r.biCanh = &p
				r.paramsJSON, _ = json.Marshal(struct {
					bicanh.Params
					runSnapshot
				}{p, runSnapshot{MinElapsedBP: r.MinElapsedBP, MinRewardTicks: r.MinRewardTicks}})
			}
		}
		if len(r.Bands) == 0 {
			fail("realm %q: need at least one band", r.ID)
		}
		seen := map[string]bool{}
		for j, b := range r.Bands {
			if !slugRe.MatchString(b.ID) || seen[b.ID] || b.Name == "" {
				fail("realm %q: band[%d]: invalid/duplicate id or empty name", r.ID, j)
			}
			seen[b.ID] = true
			if b.MinScore < 1 || (j > 0 && b.MinScore <= r.Bands[j-1].MinScore) {
				fail("realm %q: band %q: min_score must be ≥ 1 and increase", r.ID, b.ID)
			}
			if b.MinQi < 1 || b.MinKills < 1 || (j > 0 && (b.MinQi < r.Bands[j-1].MinQi || b.MinKills < r.Bands[j-1].MinKills)) {
				fail("realm %q: band %q: min_qi and min_kills must be ≥ 1 and never decrease", r.ID, b.ID)
			}
			l, ok := r.Loot[b.ID]
			if !ok {
				fail("realm %q: band %q has no loot", r.ID, b.ID)
				continue
			}
			if l.SpiritStone.Min < 0 || l.SpiritStone.Max < l.SpiritStone.Min || l.SpiritStone.Max > 10_000 {
				fail("realm %q: loot %q: spirit_stone needs 0 ≤ min ≤ max ≤ 10000", r.ID, b.ID)
			}
			if l.ArtifactChanceBP < 0 || l.ArtifactChanceBP > fixedpoint.BPScale {
				fail("realm %q: loot %q: artifact_chance_bp out of range", r.ID, b.ID)
			}
			for _, d := range l.Items {
				if d.ItemID == "" || d.Min < 0 || d.Max < d.Min || d.Max > 1000 {
					fail("realm %q: loot %q: item %q needs 0 ≤ min ≤ max ≤ 1000", r.ID, b.ID, d.ItemID)
				}
			}
			// A better band never pays less (monotone loot).
			if j > 0 {
				prev := r.Loot[r.Bands[j-1].ID]
				if l.SpiritStone.Min < prev.SpiritStone.Min || l.SpiritStone.Max < prev.SpiritStone.Max || l.ArtifactChanceBP < prev.ArtifactChanceBP {
					fail("realm %q: loot %q pays less than the band below", r.ID, b.ID)
				}
			}
		}
		for id := range r.Loot {
			if !seen[id] {
				fail("realm %q: loot %q has no band", r.ID, id)
			}
		}
		if r.Survived.SpiritStone < 0 || r.Survived.SpiritStone > 10_000 || r.Survived.ArtifactChanceBP < 0 || r.Survived.ArtifactChanceBP > fixedpoint.BPScale {
			fail("realm %q: survived bonus out of range", r.ID)
		}
		arts := map[string]bool{}
		for _, a := range r.Artifacts {
			if a.ItemID == "" || arts[a.ItemID] || a.Weight < 1 || a.Weight > 10_000 || a.DuplicateStones < 0 || a.DuplicateStones > 10_000 {
				fail("realm %q: artifact %q: need a unique item_id, weight 1..10000, duplicate_stones 0..10000", r.ID, a.ItemID)
			}
			arts[a.ItemID] = true
			r.artWeight += a.Weight
		}
		maxChance := r.Survived.ArtifactChanceBP
		for _, l := range r.Loot {
			maxChance = max(maxChance, l.ArtifactChanceBP+r.Survived.ArtifactChanceBP)
		}
		if maxChance > 0 && len(r.Artifacts) == 0 {
			fail("realm %q: artifact chance without an artifacts table", r.ID)
		}
		if maxChance > fixedpoint.BPScale {
			fail("realm %q: band + survived artifact chance exceeds 100%%", r.ID)
		}
		cp := r.DailyCaps
		if cp.SpiritStone < 0 || cp.SpiritStone > 100_000 || cp.Materials < 0 || cp.Materials > 10_000 || cp.Artifacts < 0 || cp.Artifacts > 10 {
			fail("realm %q: daily_caps out of range", r.ID)
		}
		c.realms = append(c.realms, r)
		c.byID[r.ID] = r
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("secretrealm: invalid catalogue: %w", errors.Join(errs...))
	}
	return c, nil
}

func strictParams(n *yaml.Node, into any) error {
	b, err := yaml.Marshal(n)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	return dec.Decode(into)
}

// All returns the realms in file order. Do not mutate.
func (c *Catalogue) All() []*Realm { return c.realms }

// Get returns a realm by ID.
func (c *Catalogue) Get(id string) (*Realm, bool) { r, ok := c.byID[id]; return r, ok }

// ItemIDs returns every item a realm can grant (materials and artifacts) for content cross-checks.
func (r *Realm) ItemIDs() (materials, artifacts []string) {
	seen := map[string]bool{}
	for _, b := range r.Bands {
		for _, d := range r.Loot[b.ID].Items {
			if !seen[d.ItemID] {
				seen[d.ItemID] = true
				materials = append(materials, d.ItemID)
			}
		}
	}
	for _, a := range r.Artifacts {
		artifacts = append(artifacts, a.ItemID)
	}
	return materials, artifacts
}

// ───── verdict (pure) ─────

// Verdicts and non-engine reasons. Part of the API — never rename. pass = survived the time limit, fail = fell or
// stopped (both earn loot by band when long enough), rejected = log not accepted, expired = not submitted in time.
const (
	VerdictPass     = "pass"
	VerdictFail     = "fail"
	VerdictRejected = "rejected"
	VerdictExpired  = "expired"

	ReasonLate        = "late"
	ReasonAbandoned   = "abandoned"
	ReasonInvalidLog  = "invalid_log"
	ReasonImplausible = "implausible_score"
	ReasonInputRate   = "implausible_input"
	ReasonTooFast     = "too_fast"
	ReasonMismatch    = "mismatch"
)

// Claim is what the client submits: its movement log and the end state its own simulator reached. Only the log is
// used to decide; the rest must match the server's replay exactly.
type Claim struct {
	EndTick   int64        `json:"endTick"`
	Kills     int64        `json:"kills"`
	Score     int64        `json:"score"`
	StateHash uint32       `json:"stateHash"`
	Moves     []bicanh.Run `json:"moves"` // RLE: [dir 0..8, ticks]
}

// Outcome is the server replay's result, persisted in secret_realm_runs.result.
type Outcome struct {
	Won        bool   `json:"won"`
	Reason     string `json:"reason"`
	EndTick    int64  `json:"endTick"`
	GameMillis int64  `json:"gameMillis"`
	Kills      int64  `json:"kills"`
	Score      int64  `json:"score"`
	Qi         int64  `json:"qi"`
	Level      int64  `json:"level"`
	HPLeft     int64  `json:"hpLeft"`
	Spawned    int64  `json:"spawned"`
	StateHash  uint32 `json:"stateHash"`
	// Cadence telemetry for review.
	Runs             int      `json:"runs"`
	Changes          int64    `json:"changes"`
	MaxWindowChanges int64    `json:"maxWindowChanges"`
	ChangesPerSecMil int64    `json:"changesPerSecMilli"`
	MovingTicks      int64    `json:"movingTicks"`
	ClaimedEndTick   int64    `json:"claimedEndTick"`
	ElapsedMillis    int64    `json:"elapsedMillis"`
	SlowSubmit       bool     `json:"slowSubmit,omitempty"`
	ReviewFlags      []string `json:"reviewFlags,omitempty"` // mostly_idle | low_change | periodic (review only)
}

// Run is what Judge needs to know about the issued run.
type Run struct {
	Seed      uint32
	Params    json.RawMessage
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// Judgement is the verdict of one submission.
type Judgement struct {
	Verdict string
	Reason  string
	Outcome *Outcome
}

// snapshot reads the run-level rules stored with the params (falls back to the realm's).
func snapshot(r *Realm, params json.RawMessage) runSnapshot {
	s := runSnapshot{MinElapsedBP: r.MinElapsedBP, MinRewardTicks: r.MinRewardTicks}
	var got struct {
		MinElapsedBP   *int64 `json:"minElapsedBp"`
		MinRewardTicks *int64 `json:"minRewardTicks"`
	}
	if json.Unmarshal(params, &got) == nil {
		if got.MinElapsedBP != nil && *got.MinElapsedBP >= 5000 && *got.MinElapsedBP <= fixedpoint.BPScale {
			s.MinElapsedBP = *got.MinElapsedBP
		}
		if got.MinRewardTicks != nil && *got.MinRewardTicks >= 1 {
			s.MinRewardTicks = *got.MinRewardTicks
		}
	}
	return s
}

// Judge decides a submission at server time now. Pure. Order: late → invalid log → implausible score → too fast →
// implausible input (replay) → mismatch → the game's outcome.
func Judge(r *Realm, run Run, c Claim, now time.Time) Judgement {
	if !now.Before(run.ExpiresAt) {
		return Judgement{Verdict: VerdictExpired, Reason: ReasonLate}
	}
	var p bicanh.Params
	if err := json.Unmarshal(run.Params, &p); err != nil || p.Validate() != nil {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonInvalidLog}
	}
	if c.EndTick <= 0 || c.EndTick > p.TimeLimitTicks || len(c.Moves) == 0 || len(c.Moves) > p.MaxRuns {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonInvalidLog}
	}
	// Kills are bounded by spawns: ≤ wave_max per tick plus the elites.
	if c.Kills < 0 || c.Kills > c.EndTick*p.Spawn.WaveMax+int64(len(p.Enemies))*8 || c.Score < 0 || c.Score > p.MaxScore(c.Kills) {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonImplausible}
	}
	snap := snapshot(r, run.Params)
	gameMillis := c.EndTick * p.TickMS
	elapsed := now.Sub(run.IssuedAt).Milliseconds()
	if elapsed < fixedpoint.MulBP(gameMillis, snap.MinElapsedBP) {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonTooFast}
	}
	slow := elapsed > gameMillis+SlowSubmitSlackMillis
	res, err := bicanh.Replay(&p, run.Seed, c.Moves, c.EndTick)
	if errors.Is(err, bicanh.ErrImplausibleInput) {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonInputRate, Outcome: &Outcome{Reason: ReasonInputRate, Runs: res.Runs,
			Changes: res.Changes, MaxWindowChanges: res.MaxWindowChanges, MovingTicks: res.MovingTicks, ClaimedEndTick: c.EndTick,
			GameMillis: gameMillis, ElapsedMillis: elapsed, SlowSubmit: slow, ChangesPerSecMil: perSecMilli(res.Changes, gameMillis)}}
	}
	if err != nil {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonInvalidLog}
	}
	o := &Outcome{Won: res.Won, Reason: res.Reason, EndTick: res.EndTick, GameMillis: res.EndTick * p.TickMS, Kills: res.Kills, Score: res.Score,
		Qi: res.Qi, Level: res.Level, HPLeft: res.HPLeft, Spawned: res.Spawned, StateHash: res.StateHash, Runs: res.Runs, Changes: res.Changes,
		MaxWindowChanges: res.MaxWindowChanges, MovingTicks: res.MovingTicks, ClaimedEndTick: c.EndTick, ElapsedMillis: elapsed, SlowSubmit: slow,
		ChangesPerSecMil: perSecMilli(res.Changes, res.EndTick*p.TickMS), ReviewFlags: ReviewFlags(c.Moves, c.EndTick, p.TickMS)}
	if res.Reason == bicanh.ReasonEndedEarly || res.EndTick != c.EndTick || res.Kills != c.Kills || res.Score != c.Score || res.StateHash != c.StateHash {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonMismatch, Outcome: o}
	}
	if res.Won {
		return Judgement{Verdict: VerdictPass, Reason: res.Reason, Outcome: o}
	}
	return Judgement{Verdict: VerdictFail, Reason: res.Reason, Outcome: o}
}

func perSecMilli(n int64, gameMillis int64) int64 {
	if gameMillis <= 0 {
		return 0
	}
	return n * 1_000_000 / gameMillis
}

// BandFor returns the band of a judged run (nil = no reward): only pass/fail verdicts, at least min_reward_ticks
// played, and the highest band whose min_score the replayed score reaches.
func BandFor(r *Realm, params json.RawMessage, verdict string, o *Outcome) *Band {
	if o == nil || (verdict != VerdictPass && verdict != VerdictFail) || o.EndTick < snapshot(r, params).MinRewardTicks {
		return nil
	}
	var got *Band
	for i := range r.Bands {
		b := &r.Bands[i]
		if o.Score >= b.MinScore && o.Qi >= b.MinQi && o.Kills >= b.MinKills {
			got = b
		}
	}
	return got
}

// Review flags (telemetry only, never a rejection): patterns a blind script produces.
const (
	FlagMostlyIdle = "mostly_idle" // moving < 20% of the ticks
	FlagLowChange  = "low_change"  // < 6 direction changes per minute
	FlagPeriodic   = "periodic"    // ≥ 8 runs, ≥ 80% share one length (±1 tick) AND the directions repeat a cycle ≤ 8: a loop
)

// ReviewFlags inspects a move log for scripted-looking patterns.
func ReviewFlags(moves []bicanh.Run, endTick, tickMS int64) []string {
	var flags []string
	var moving int64
	lengths := map[int64]int{}
	for _, m := range moves {
		if m[0] != 0 {
			moving += m[1]
		}
		lengths[m[1]]++
	}
	if endTick > 0 && moving*5 < endTick {
		flags = append(flags, FlagMostlyIdle)
	}
	if minutes := endTick * tickMS; minutes > 0 && int64(len(moves)-1)*60_000 < 6*minutes {
		flags = append(flags, FlagLowChange)
	}
	if len(moves) >= 8 {
		best := 0
		for l := range lengths {
			best = max(best, lengths[l-1]+lengths[l]+lengths[l+1])
		}
		cyclic := false
		for k := 1; k <= 8 && k < len(moves) && !cyclic; k++ {
			same := 0
			for i := k; i < len(moves); i++ {
				if moves[i][0] == moves[i-k][0] {
					same++
				}
			}
			cyclic = same*5 >= (len(moves)-k)*4
		}
		if best*5 >= len(moves)*4 && cyclic {
			flags = append(flags, FlagPeriodic)
		}
	}
	return flags
}

// ───── loot (pure) ─────

// RNG is injected so loot is deterministic in tests and auditable in prod (math/rand/v2 seeded from crypto/rand).
type RNG interface {
	IntN(n int) int
}

// Today is what the character already received from secret realms on the game day (under the day-row lock).
type Today struct {
	SpiritStone int64
	Materials   int64
	Artifacts   int64
}

// Grant is one granted item.
type Grant struct {
	ItemID   string `json:"itemId"`
	Quantity int64  `json:"quantity"`
}

// Loot is what one run grants, after the daily caps. Rolled = before caps (disclosed, audit).
type Loot struct {
	Band             string  `json:"band"`
	SpiritStone      int64   `json:"spiritStone"`
	Items            []Grant `json:"items"`
	Artifact         string  `json:"artifact,omitempty"`       // pháp bảo granted
	DuplicateOf      string  `json:"duplicateOf,omitempty"`    // rolled pháp bảo already owned → duplicate stones
	ArtifactChanceBP int64   `json:"artifactChanceBp"`         // the disclosed chance this run had
	RolledStone      int64   `json:"rolledSpiritStone"`        // incl. duplicate stones, before the cap
	RolledItems      []Grant `json:"rolledItems"`              //
	RolledArtifact   string  `json:"rolledArtifact,omitempty"` // before the artifact cap
	Clipped          bool    `json:"clipped"`                  // a daily cap reduced something
	ArtifactCapped   bool    `json:"artifactCapped,omitempty"` // the pháp bảo was lost to the daily cap
}

func rollRange(rng RNG, lo, hi int64) int64 {
	if hi <= lo {
		return lo
	}
	return lo + int64(rng.IntN(int(hi-lo+1)))
}

// RollLoot draws the loot of a band (+ survived bonus) and applies the daily caps. RNG draws, in order: stones, each
// item drop, the artifact chance, then the artifact pick (only when the chance hit). owned reports pháp bảo the
// character already holds (a duplicate becomes duplicate_stones).
func RollLoot(r *Realm, band *Band, survived bool, rng RNG, owned func(itemID string) bool, today Today) Loot {
	l := Loot{Band: band.ID, Items: []Grant{}, RolledItems: []Grant{}}
	bl := r.Loot[band.ID]
	stones := rollRange(rng, bl.SpiritStone.Min, bl.SpiritStone.Max)
	chance := bl.ArtifactChanceBP
	if survived {
		stones += r.Survived.SpiritStone
		chance += r.Survived.ArtifactChanceBP
	}
	l.ArtifactChanceBP = chance
	for _, d := range bl.Items {
		if q := rollRange(rng, d.Min, d.Max); q > 0 {
			l.RolledItems = append(l.RolledItems, Grant{ItemID: d.ItemID, Quantity: q})
		}
	}
	if chance > 0 && int64(rng.IntN(fixedpoint.BPScale)) < chance && r.artWeight > 0 {
		pick := int64(rng.IntN(int(r.artWeight)))
		for _, a := range r.Artifacts {
			if pick < a.Weight {
				if owned != nil && owned(a.ItemID) {
					l.DuplicateOf = a.ItemID
					stones += a.DuplicateStones
				} else {
					l.RolledArtifact = a.ItemID
				}
				break
			}
			pick -= a.Weight
		}
	}
	l.RolledStone = stones
	// Daily caps.
	cp := r.DailyCaps
	l.SpiritStone = min(stones, max(0, cp.SpiritStone-today.SpiritStone))
	room := max(0, cp.Materials-today.Materials)
	for _, g := range l.RolledItems {
		q := min(g.Quantity, room)
		room -= q
		if q > 0 {
			l.Items = append(l.Items, Grant{ItemID: g.ItemID, Quantity: q})
		}
		if q < g.Quantity {
			l.Clipped = true
		}
	}
	if l.RolledArtifact != "" {
		if today.Artifacts < cp.Artifacts {
			l.Artifact = l.RolledArtifact
		} else {
			l.ArtifactCapped, l.Clipped = true, true
		}
	}
	if l.SpiritStone < stones {
		l.Clipped = true
	}
	return l
}

// Materials is the total quantity of non-artifact items granted.
func (l Loot) Materials() int64 {
	var n int64
	for _, g := range l.Items {
		n += g.Quantity
	}
	return n
}

// ArtifactCount is 1 when a pháp bảo was granted.
func (l Loot) ArtifactCount() int64 {
	if l.Artifact != "" {
		return 1
	}
	return 0
}
