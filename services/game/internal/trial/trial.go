// Package trial is the mini-game trial framework (Phase 3.1, decisions D6): server-issued seeded runs, a
// single-use submission verified by deterministic replay of the client's input log, a lose cooldown and a pass
// that gates a major breakthrough (Phase 3.2: Tâm Ma gates Kim Đan → Nguyên Anh; Phase 3.3: Hóa Phàm gates
// Nguyên Anh → Hóa Thần).
//
// trial.go holds the catalogue (content/catalog/trials.yaml) and the pure verdict rule (Judge); service.go owns the
// transactions; the simulators live in sub-packages (tamma, hoapham).
package trial

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
	"tiendao/game/internal/trial/hoapham"
	"tiendao/game/internal/trial/tamma"
)

// Engines implemented by this server.
const (
	EngineTamMa   = "tam_ma"
	EngineHoaPham = "hoa_pham"
)

// MaxClaimBytes is the request-body limit of POST /trials/{runId}/submit (httpapi uses it). The loader checks
// that the largest log of every kind fits:
//   - tâm ma: max_inputs × ClaimBytesPerInput (each [tick,action] pair);
//   - hóa phàm: time_limit_ticks × ClaimBytesPerPoint (one sampled point per pressed tick, a "dx,dy," pair with
//     |d| ≤ max_step) + max_strokes × ClaimBytesPerStroke (the "[start,x,y]" head of each stroke).
const (
	MaxClaimBytes       = 96 << 10
	ClaimBytesPerInput  = 12
	ClaimBytesPerPoint  = 9
	ClaimBytesPerStroke = 24
	claimOverheadBytes  = 512
)

// OpensAt values.
const (
	OpensAtLastStage = "last_stage"
	OpensAtRealm     = "realm"
)

// Gate names the major breakthrough a kind guards: the one LEAVING Tier (on every path).
type Gate struct {
	Tier int `yaml:"tier" json:"tier"`
}

// Kind is one trial type. Immutable after load.
type Kind struct {
	ID                  string    `yaml:"id" json:"kind"`
	Name                string    `yaml:"name" json:"name"`
	Summary             string    `yaml:"summary" json:"summary"`
	Engine              string    `yaml:"engine" json:"engine"`
	Gate                Gate      `yaml:"gate" json:"gate"`
	OpensAt             string    `yaml:"opens_at" json:"opensAt"`
	ExpireAfterSeconds  int64     `yaml:"expire_after_seconds" json:"expireAfterSeconds"`
	MinElapsedBP        int64     `yaml:"min_elapsed_bp" json:"minElapsedBp"`
	LoseCooldownSeconds int64     `yaml:"lose_cooldown_seconds" json:"loseCooldownSeconds"`
	PassValidSeconds    int64     `yaml:"pass_valid_seconds" json:"passValidSeconds"`
	Params              yaml.Node `yaml:"params" json:"-"`

	tamMa      *tamma.Params
	hoaPham    *hoapham.Params
	paramsJSON json.RawMessage
}

// ParamsJSON is the snapshot stored in each run and sent to the client (camelCase): the engine params plus the
// kind's minElapsedBp, so a catalogue change never alters how a run in flight is judged.
func (k *Kind) ParamsJSON() json.RawMessage { return k.paramsJSON }

// TamMa returns the decoded Tâm Ma params (nil for other engines).
func (k *Kind) TamMa() *tamma.Params { return k.tamMa }

// HoaPham returns the decoded Hóa Phàm params (nil for other engines).
func (k *Kind) HoaPham() *hoapham.Params { return k.hoaPham }

// Catalogue holds the trial kinds.
type Catalogue struct {
	kinds  []*Kind
	byID   map[string]*Kind
	byTier map[int]*Kind
}

type file struct {
	Version int     `yaml:"version"`
	Kinds   []*Kind `yaml:"kinds"`
}

var slugRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// LoadFile parses and validates trials.yaml.
func LoadFile(path string) (*Catalogue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load parses and validates trial YAML.
func Load(r io.Reader) (*Catalogue, error) {
	var raw file
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("trial: decode: %w", err)
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if raw.Version != 1 {
		fail("unsupported version %d", raw.Version)
	}
	c := &Catalogue{byID: map[string]*Kind{}, byTier: map[int]*Kind{}}
	for i, k := range raw.Kinds {
		if !slugRe.MatchString(k.ID) {
			fail("kind[%d]: invalid id %q", i, k.ID)
			continue
		}
		if _, dup := c.byID[k.ID]; dup {
			fail("kind %q: duplicate id", k.ID)
		}
		if k.Name == "" || k.Summary == "" {
			fail("kind %q: empty name or summary", k.ID)
		}
		if k.Gate.Tier < 1 {
			fail("kind %q: gate.tier must be ≥ 1", k.ID)
		} else if prev, dup := c.byTier[k.Gate.Tier]; dup {
			fail("kind %q: tier %d is already gated by %q", k.ID, k.Gate.Tier, prev.ID)
		}
		if k.OpensAt != OpensAtLastStage && k.OpensAt != OpensAtRealm {
			fail("kind %q: opens_at must be %s or %s", k.ID, OpensAtLastStage, OpensAtRealm)
		}
		if k.ExpireAfterSeconds < 60 || k.ExpireAfterSeconds > 3600 {
			fail("kind %q: expire_after_seconds must be within 60..3600", k.ID)
		}
		if k.MinElapsedBP < 5000 || k.MinElapsedBP > fixedpoint.BPScale {
			fail("kind %q: min_elapsed_bp must be within 5000..10000", k.ID)
		}
		if k.LoseCooldownSeconds < 60 || k.LoseCooldownSeconds > 86400 {
			fail("kind %q: lose_cooldown_seconds must be within 60..86400", k.ID)
		}
		if k.PassValidSeconds < 3600 || k.PassValidSeconds > 30*86400 {
			fail("kind %q: pass_valid_seconds must be within 1 h..30 days", k.ID)
		}
		switch k.Engine {
		case EngineTamMa:
			var p tamma.Params
			if err := strictParams(&k.Params, &p); err != nil {
				fail("kind %q: params: %v", k.ID, err)
				break
			}
			if err := p.Validate(); err != nil {
				fail("kind %q: params: %w", k.ID, err)
				break
			}
			if int64(p.MaxInputs)*ClaimBytesPerInput+claimOverheadBytes > MaxClaimBytes {
				fail("kind %q: max_inputs × %d bytes exceeds the submit body limit (%d bytes)", k.ID, ClaimBytesPerInput, MaxClaimBytes)
			}
			// The whole game (incl. pauses within reason) must fit before the run expires.
			if gameSec := p.TimeLimitTicks * p.TickMS / 1000; gameSec+60 > k.ExpireAfterSeconds {
				fail("kind %q: expire_after_seconds must be ≥ time limit (%d s) + 60", k.ID, gameSec)
			}
			k.tamMa = &p
			k.paramsJSON, _ = json.Marshal(struct {
				tamma.Params
				runSnapshot
			}{p, runSnapshot{MinElapsedBP: k.MinElapsedBP}})
		case EngineHoaPham:
			var p hoapham.Params
			if err := strictParams(&k.Params, &p); err != nil {
				fail("kind %q: params: %v", k.ID, err)
				break
			}
			if err := p.Validate(); err != nil {
				fail("kind %q: params: %w", k.ID, err)
				break
			}
			if p.TimeLimitTicks*ClaimBytesPerPoint+int64(p.MaxStrokes)*ClaimBytesPerStroke+claimOverheadBytes > MaxClaimBytes {
				fail("kind %q: the longest stroke log exceeds the submit body limit (%d bytes)", k.ID, MaxClaimBytes)
			}
			if gameSec := p.TimeLimitTicks * p.TickMS / 1000; gameSec+60 > k.ExpireAfterSeconds {
				fail("kind %q: expire_after_seconds must be ≥ time limit (%d s) + 60", k.ID, gameSec)
			}
			k.hoaPham = &p
			k.paramsJSON, _ = json.Marshal(struct {
				hoapham.Params
				runSnapshot
			}{p, runSnapshot{MinElapsedBP: k.MinElapsedBP}})
		default:
			fail("kind %q: unknown engine %q", k.ID, k.Engine)
		}
		c.kinds = append(c.kinds, k)
		c.byID[k.ID] = k
		if k.Gate.Tier >= 1 {
			if _, dup := c.byTier[k.Gate.Tier]; !dup {
				c.byTier[k.Gate.Tier] = k
			}
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("trial: invalid catalogue: %w", errors.Join(errs...))
	}
	return c, nil
}

// runSnapshot holds the kind-level rules snapshotted next to the engine params in every run.
type runSnapshot struct {
	MinElapsedBP int64 `json:"minElapsedBp"`
}

// minElapsedBP is the run's snapshotted min_elapsed_bp; runs issued before it was snapshotted (or with an
// out-of-range value) fall back to the catalogue's.
func minElapsedBP(k *Kind, params json.RawMessage) int64 {
	var s struct {
		MinElapsedBP *int64 `json:"minElapsedBp"`
	}
	if json.Unmarshal(params, &s) == nil && s.MinElapsedBP != nil && *s.MinElapsedBP >= 5000 && *s.MinElapsedBP <= fixedpoint.BPScale {
		return *s.MinElapsedBP
	}
	return k.MinElapsedBP
}

// strictParams re-decodes the params node with KnownFields so a typo inside params fails loudly too.
func strictParams(n *yaml.Node, into any) error {
	b, err := yaml.Marshal(n)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	return dec.Decode(into)
}

// All returns the kinds in file order. Do not mutate.
func (c *Catalogue) All() []*Kind { return c.kinds }

// Get returns a kind by ID.
func (c *Catalogue) Get(id string) (*Kind, bool) { k, ok := c.byID[id]; return k, ok }

// GateFor returns the kind that guards the major breakthrough leaving tier, if any.
func (c *Catalogue) GateFor(tier int) (*Kind, bool) {
	if c == nil {
		return nil, false
	}
	k, ok := c.byTier[tier]
	return k, ok
}

// ───── verdict (pure) ─────

// Verdicts and non-engine reasons. Part of the API — never rename.
const (
	VerdictPass     = "pass"
	VerdictFail     = "fail"
	VerdictRejected = "rejected"
	VerdictExpired  = "expired"

	ReasonLate        = "late"              // submitted after expires_at
	ReasonAbandoned   = "abandoned"         // never submitted; closed when the next run was requested
	ReasonInvalidLog  = "invalid_log"       // malformed input log
	ReasonImplausible = "implausible_score" // claimed lines/score impossible for the claimed length
	ReasonInputRate   = "implausible_input" // inputs faster than a human (input window cap)
	ReasonTooFast     = "too_fast"          // less server time elapsed than the simulated game takes
	ReasonMismatch    = "mismatch"          // the replay disagrees with the claimed end state
)

// Claim is what the client submits: its input log and the end state its own simulator reached. Only the log is
// used to decide; the rest must match the server's replay exactly. Lines = rows cleared (tâm ma) or knots severed
// (hóa phàm). The log is Inputs (tâm ma) or Strokes (hóa phàm); the other one must be empty.
type Claim struct {
	EndTick   int64            `json:"endTick"`
	Lines     int64            `json:"lines"`
	Score     int64            `json:"score"`
	StateHash uint32           `json:"stateHash"`
	Inputs    [][2]int64       `json:"inputs"`            // tâm ma: [tick, action]
	Strokes   []hoapham.Stroke `json:"strokes,omitempty"` // hóa phàm: [start, x0, y0, dx1, dy1, …]
}

// Outcome is the server replay's result, persisted in trial_runs.result.
type Outcome struct {
	Won        bool   `json:"won"`
	Reason     string `json:"reason"`
	EndTick    int64  `json:"endTick"`
	GameMillis int64  `json:"gameMillis"`
	Lines      int64  `json:"lines"`
	Score      int64  `json:"score"`
	Pieces     int64  `json:"pieces"`
	StateHash  uint32 `json:"stateHash"`
	// Cadence telemetry for review (persisted in trial_runs.result; returned only with a pass/fail outcome).
	Inputs            int   `json:"inputs"`
	InputsPerSecMilli int64 `json:"inputsPerSecMilli"`
	MaxWindowInputs   int   `json:"maxWindowInputs"`
	MinLockGapTicks   int64 `json:"minLockGapTicks"`
	ClaimedEndTick    int64 `json:"claimedEndTick"`
	// Hóa phàm telemetry (Inputs = sampled pointer points; Lines = knots severed; Pieces = objects spawned).
	Strokes          int   `json:"strokes,omitempty"`
	MaxStep          int64 `json:"maxStep,omitempty"`
	MaxWindowTravel  int64 `json:"maxWindowTravel,omitempty"`
	MaxWindowStrokes int   `json:"maxWindowStrokes,omitempty"`
	LanternsCut      int64 `json:"lanternsCut,omitempty"`
	Escaped          int64 `json:"escaped,omitempty"`
	BestCombo        int64 `json:"bestCombo,omitempty"`
	// Server time between issue and submit, and whether it exceeded the game time + SlowSubmitSlackMillis (possible
	// replay-shopping of a known seed; review flag, never a rejection). Hóa phàm only for now.
	ElapsedMillis int64 `json:"elapsedMillis,omitempty"`
	SlowSubmit    bool  `json:"slowSubmit,omitempty"`
}

// SlowSubmitSlackMillis is the generous slack (loading, pauses, network) before a submit is flagged as slow.
const SlowSubmitSlackMillis = 60_000

// Run is what Judge needs to know about the issued run.
type Run struct {
	Seed      uint32
	Params    json.RawMessage // snapshot taken at issue time
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// Judgement is the verdict of one submission.
type Judgement struct {
	Verdict string
	Reason  string
	Outcome *Outcome // nil when the run was not replayed (late, invalid, too fast…)
}

// Judge decides a submission at server time now. Pure. Order: late → invalid log → implausible score → too fast
// (server clock vs. simulated time) → replay mismatch → the game's own outcome.
func Judge(k *Kind, run Run, c Claim, now time.Time) Judgement {
	if !now.Before(run.ExpiresAt) {
		return Judgement{Verdict: VerdictExpired, Reason: ReasonLate}
	}
	switch k.Engine {
	case EngineTamMa:
		return judgeTamMa(k, run, c, now)
	case EngineHoaPham:
		return judgeHoaPham(k, run, c, now)
	}
	return Judgement{Verdict: VerdictRejected, Reason: ReasonInvalidLog}
}

func judgeTamMa(k *Kind, run Run, c Claim, now time.Time) Judgement {
	var p tamma.Params
	if err := json.Unmarshal(run.Params, &p); err != nil || p.Validate() != nil {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonInvalidLog}
	}
	if c.EndTick <= 0 || c.EndTick > p.TimeLimitTicks || len(c.Inputs) > p.MaxInputs || len(c.Strokes) > 0 {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonInvalidLog}
	}
	// Lines first (bounds MaxScore): a lock clears ≤ 5 rows and locks are ≥ min gravity/lock-gap apart, so even one
	// line per tick on average is impossible.
	if c.Lines < 0 || c.Lines > c.EndTick || c.Score < 0 || c.Score > p.MaxScore(c.Lines) {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonImplausible}
	}
	gameMillis := c.EndTick * p.TickMS
	elapsed := now.Sub(run.IssuedAt).Milliseconds()
	if elapsed < fixedpoint.MulBP(gameMillis, minElapsedBP(k, run.Params)) {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonTooFast}
	}
	inputs := make([]tamma.Input, len(c.Inputs))
	for i, in := range c.Inputs {
		if in[1] < 0 || in[1] > 255 {
			return Judgement{Verdict: VerdictRejected, Reason: ReasonInvalidLog}
		}
		inputs[i] = tamma.Input{Tick: in[0], Action: tamma.Action(in[1])}
	}
	r, err := tamma.Replay(&p, run.Seed, inputs, c.EndTick)
	if errors.Is(err, tamma.ErrImplausibleInput) {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonInputRate, Outcome: &Outcome{Reason: ReasonInputRate,
			Inputs: r.Inputs, MaxWindowInputs: r.MaxWindowInputs, ClaimedEndTick: c.EndTick, GameMillis: gameMillis,
			InputsPerSecMilli: perSecMilli(r.Inputs, gameMillis)}}
	}
	if err != nil {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonInvalidLog}
	}
	o := &Outcome{Won: r.Won, Reason: r.Reason, EndTick: r.EndTick, GameMillis: r.EndTick * p.TickMS, Lines: r.Lines,
		Score: r.Score, Pieces: r.Pieces, StateHash: r.StateHash, Inputs: r.Inputs, MaxWindowInputs: r.MaxWindowInputs,
		MinLockGapTicks: r.MinLockGapTicks, ClaimedEndTick: c.EndTick, InputsPerSecMilli: perSecMilli(r.Inputs, r.EndTick*p.TickMS)}
	if r.Reason == tamma.ReasonEndedEarl || r.EndTick != c.EndTick || r.Lines != c.Lines || r.Score != c.Score || r.StateHash != c.StateHash {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonMismatch, Outcome: o}
	}
	if r.Won {
		return Judgement{Verdict: VerdictPass, Reason: r.Reason, Outcome: o}
	}
	return Judgement{Verdict: VerdictFail, Reason: r.Reason, Outcome: o}
}

// judgeHoaPham: same order as tâm ma — invalid log → implausible score → too fast → implausible pointer input →
// mismatch → the game's outcome.
func judgeHoaPham(k *Kind, run Run, c Claim, now time.Time) Judgement {
	var p hoapham.Params
	if err := json.Unmarshal(run.Params, &p); err != nil || p.Validate() != nil {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonInvalidLog}
	}
	if c.EndTick <= 0 || c.EndTick > p.TimeLimitTicks || len(c.Strokes) > p.MaxStrokes || len(c.Inputs) > 0 {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonInvalidLog}
	}
	// A knot is severed at most once and at most one wave of ≤ wave_max knots spawns per tick.
	if c.Lines < 0 || c.Lines > c.EndTick*p.Spawn.WaveMax || c.Score < 0 || c.Score > p.MaxScore(c.Lines) {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonImplausible}
	}
	gameMillis := c.EndTick * p.TickMS
	elapsed := now.Sub(run.IssuedAt).Milliseconds()
	if elapsed < fixedpoint.MulBP(gameMillis, minElapsedBP(k, run.Params)) {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonTooFast}
	}
	// Seed foreknowledge (anti-cheat M1): the seed is known at start, so a long gap between issue and submit may be
	// local retries of the same seed. Not rejected (pauses and slow networks are legitimate) — flagged for review.
	slow := elapsed > gameMillis+SlowSubmitSlackMillis
	r, err := hoapham.Replay(&p, run.Seed, c.Strokes, c.EndTick)
	if errors.Is(err, hoapham.ErrImplausibleInput) {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonInputRate, Outcome: &Outcome{Reason: ReasonInputRate,
			ElapsedMillis: elapsed, SlowSubmit: slow, Inputs: int(r.Points), Strokes: r.Strokes, MaxStep: r.MaxStep, MaxWindowTravel: r.MaxWindowTravel, MaxWindowStrokes: r.MaxWindowStrokes,
			ClaimedEndTick: c.EndTick, GameMillis: gameMillis, InputsPerSecMilli: perSecMilli(int(r.Points), gameMillis)}}
	}
	if err != nil {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonInvalidLog}
	}
	o := &Outcome{Won: r.Won, Reason: r.Reason, EndTick: r.EndTick, GameMillis: r.EndTick * p.TickMS, Lines: r.Cuts, Score: r.Score,
		Pieces: r.Spawned, StateHash: r.StateHash, Inputs: int(r.Points), InputsPerSecMilli: perSecMilli(int(r.Points), r.EndTick*p.TickMS),
		ClaimedEndTick: c.EndTick, Strokes: r.Strokes, MaxStep: r.MaxStep, MaxWindowTravel: r.MaxWindowTravel,
		MaxWindowStrokes: r.MaxWindowStrokes, LanternsCut: r.LanternsCut, Escaped: r.Escaped, BestCombo: r.BestCombo,
		ElapsedMillis: elapsed, SlowSubmit: slow}
	if r.Reason == hoapham.ReasonEndedEarly || r.EndTick != c.EndTick || r.Cuts != c.Lines || r.Score != c.Score || r.StateHash != c.StateHash {
		return Judgement{Verdict: VerdictRejected, Reason: ReasonMismatch, Outcome: o}
	}
	if r.Won {
		return Judgement{Verdict: VerdictPass, Reason: r.Reason, Outcome: o}
	}
	return Judgement{Verdict: VerdictFail, Reason: r.Reason, Outcome: o}
}

// perSecMilli = inputs per second × 1000 over the simulated game time.
func perSecMilli(inputs int, gameMillis int64) int64 {
	if gameMillis <= 0 {
		return 0
	}
	return int64(inputs) * 1_000_000 / gameMillis
}
