package secretrealm

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/db"
	"tiendao/game/internal/economy"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/realm"
	"tiendao/game/internal/vip"
)

var (
	ErrNoCharacter      = errors.New("secretrealm: character not found")
	ErrUnknownRealm     = errors.New("secretrealm: unknown realm")
	ErrRunNotFound      = errors.New("secretrealm: run not found")
	ErrAlreadySubmitted = errors.New("secretrealm: run already submitted")
	ErrKeyReused        = errors.New("secretrealm: idempotency key reused for another realm")
)

// Block codes of ErrBlocked (start refused). Part of the API.
const (
	BlockDailyLimit = "daily_limit"
	BlockLocked     = "locked" // realm below min_tier
)

// ErrBlocked: the run cannot start now; Status says why.
type ErrBlocked struct {
	Code   string
	Status RealmStatus
}

func (e *ErrBlocked) Error() string { return "secretrealm: start blocked: " + e.Code }

// TodayView is what the character used/received today from this realm (game day UTC+7).
type TodayView struct {
	GameDay     string `json:"gameDay"`
	Runs        int64  `json:"runs"`
	SpiritStone int64  `json:"spiritStone"`
	Materials   int64  `json:"materials"`
	Artifacts   int64  `json:"artifacts"`
}

// OpenRunView is the run in progress (resumable with the same seed until it expires).
type OpenRunView struct {
	RunID     uuid.UUID `json:"runId"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// RealmStatus is one realm as seen by the character (GET /secret-realms and inside every mutation response).
type RealmStatus struct {
	RealmID            string              `json:"realmId"`
	Name               string              `json:"name"`
	Summary            string              `json:"summary"`
	Unlocked           bool                `json:"unlocked"`
	MinTier            int                 `json:"minTier"`
	RunsPerDay         int64               `json:"runsPerDay"`
	RunsLeft           int64               `json:"runsLeft"`
	ExpireAfterSeconds int64               `json:"expireAfterSeconds"`
	MinRewardTicks     int64               `json:"minRewardTicks"`
	Bands              []Band              `json:"bands"`
	Loot               map[string]BandLoot `json:"loot"`
	Survived           SurvivedBonus       `json:"survived"`
	Artifacts          []ArtifactDrop      `json:"artifacts"`
	DailyCaps          Caps                `json:"dailyCaps"`
	Today              TodayView           `json:"today"`
	ResetAt            time.Time           `json:"resetAt"`
	OpenRun            *OpenRunView        `json:"openRun"`
	Params             json.RawMessage     `json:"params"`
}

// RecentRun is one judged run in the history.
type RecentRun struct {
	RunID    uuid.UUID `json:"runId"`
	RealmID  string    `json:"realmId"`
	IssuedAt time.Time `json:"issuedAt"`
	Verdict  string    `json:"verdict"`
	Reason   string    `json:"reason"`
	Band     string    `json:"band"`
	Score    int64     `json:"score"`
	Loot     *Loot     `json:"loot"`
}

// StatusView is GET /secret-realms.
type StatusView struct {
	ServerNow time.Time     `json:"serverNow"`
	Realms    []RealmStatus `json:"realms"`
	Recent    []RecentRun   `json:"recent"`
}

// StartView is POST /secret-realms/{realmId}/start.
type StartView struct {
	RunID     uuid.UUID       `json:"runId"`
	RealmID   string          `json:"realmId"`
	Seed      uint32          `json:"seed"`
	IssuedAt  time.Time       `json:"issuedAt"`
	ExpiresAt time.Time       `json:"expiresAt"`
	ServerNow time.Time       `json:"serverNow"`
	Params    json.RawMessage `json:"params"`
	Resumed   bool            `json:"resumed"`
	Status    RealmStatus     `json:"status"`
}

// SubmitView is POST /secret-realms/runs/{runId}/submit: the authoritative verdict and loot.
type SubmitView struct {
	RunID     uuid.UUID   `json:"runId"`
	RealmID   string      `json:"realmId"`
	Verdict   string      `json:"verdict"`
	Reason    string      `json:"reason"`
	Outcome   *Outcome    `json:"outcome"` // pass/fail only
	Band      *Band       `json:"band"`    // nil = no reward
	Loot      *Loot       `json:"loot"`
	VIPPoints int64       `json:"vipPoints"`
	Replayed  bool        `json:"replayed"`
	Status    RealmStatus `json:"status"`
}

// Service owns the secret realm transactions.
type Service struct {
	Pool   *pgxpool.Pool
	Cat    *Catalogue
	Realms *realm.Catalogue
	// Project returns the character's stage after the pending idle settlement, read-only (cultivation.ProjectedStage).
	Project func(ctx context.Context, q *db.Queries, userID uuid.UUID, p db.CharacterProgression) (*realm.Stage, error)
	VIP     *vip.Table // nil → no điểm duyên for surviving
	Now     func() time.Time
	Seed    func() uint32 // nil → crypto/rand
	RNG     func() RNG    // loot RNG per submit; nil → math/rand/v2 seeded from crypto/rand
}

func (s *Service) seed() uint32 {
	if s.Seed != nil {
		if v := s.Seed(); v != 0 {
			return v
		}
		return 1
	}
	var b [4]byte
	for {
		if _, err := crand.Read(b[:]); err != nil {
			panic(err)
		}
		if v := binary.LittleEndian.Uint32(b[:]); v != 0 {
			return v
		}
	}
}

func (s *Service) rng() RNG {
	if s.RNG != nil {
		return s.RNG()
	}
	var b [16]byte
	if _, err := crand.Read(b[:]); err != nil {
		panic(err)
	}
	return rand.New(rand.NewPCG(binary.LittleEndian.Uint64(b[:8]), binary.LittleEndian.Uint64(b[8:])))
}

func (s *Service) character(ctx context.Context, q *db.Queries, userID uuid.UUID, lock bool) (db.Character, db.CharacterProgression, error) {
	ch, err := q.GetCharacterByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ch, db.CharacterProgression{}, ErrNoCharacter
	} else if err != nil {
		return ch, db.CharacterProgression{}, err
	}
	var p db.CharacterProgression
	if lock {
		p, err = q.GetProgressionForUpdate(ctx, ch.ID)
	} else {
		p, err = q.GetProgression(ctx, ch.ID)
	}
	if err != nil {
		return ch, p, fmt.Errorf("progression: %w", err)
	}
	return ch, p, nil
}

// Status lists every realm for the character (read-only) and the last judged runs.
func (s *Service) Status(ctx context.Context, userID uuid.UUID) (StatusView, error) {
	q := db.New(s.Pool)
	ch, p, err := s.character(ctx, q, userID, false)
	if err != nil {
		return StatusView{}, err
	}
	stage, err := s.Project(ctx, q, userID, p)
	if err != nil {
		return StatusView{}, err
	}
	now := s.Now().UTC()
	out := StatusView{ServerNow: now, Realms: make([]RealmStatus, 0, len(s.Cat.All())), Recent: []RecentRun{}}
	for _, r := range s.Cat.All() {
		st, err := s.status(ctx, q, ch.ID, stage, r, now)
		if err != nil {
			return StatusView{}, err
		}
		out.Realms = append(out.Realms, st)
	}
	rows, err := q.ListRecentSecretRealmRuns(ctx, db.ListRecentSecretRealmRunsParams{CharacterID: ch.ID, Limit: 5})
	if err != nil {
		return StatusView{}, err
	}
	for _, row := range rows {
		rr := RecentRun{RunID: row.ID, RealmID: row.RealmID, IssuedAt: row.IssuedAt, Verdict: row.Verdict.String, Reason: row.Reason.String,
			Band: row.Band.String}
		if len(row.Loot) > 0 {
			var l Loot
			if json.Unmarshal(row.Loot, &l) == nil {
				rr.Loot = &l
			}
		}
		if len(row.Result) > 0 {
			var o Outcome
			if json.Unmarshal(row.Result, &o) == nil {
				rr.Score = o.Score
			}
		}
		out.Recent = append(out.Recent, rr)
	}
	return out, nil
}

func (s *Service) status(ctx context.Context, q *db.Queries, charID uuid.UUID, stage *realm.Stage, r *Realm, now time.Time) (RealmStatus, error) {
	st := RealmStatus{RealmID: r.ID, Name: r.Name, Summary: r.Summary, MinTier: r.MinTier, RunsPerDay: r.RunsPerDay,
		ExpireAfterSeconds: r.ExpireAfterSeconds, MinRewardTicks: r.MinRewardTicks, Bands: r.Bands, Loot: r.Loot, Survived: r.Survived,
		Artifacts: r.Artifacts, DailyCaps: r.DailyCaps, Params: r.ParamsJSON(), ResetAt: gameday.NextReset(now),
		Unlocked: stage != nil && stage.Tier >= r.MinTier}
	st.Today.GameDay = gameday.Of(now).Format(time.DateOnly)
	day, err := q.GetSecretRealmDay(ctx, db.GetSecretRealmDayParams{CharacterID: charID, GameDay: gameday.PG(now)})
	if err == nil {
		st.Today.Runs, st.Today.SpiritStone, st.Today.Materials, st.Today.Artifacts = int64(day.Runs), day.SpiritStone, day.Materials, int64(day.Artifacts)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return st, err
	}
	st.RunsLeft = max(0, r.RunsPerDay-st.Today.Runs)
	if open, err := q.GetOpenSecretRealmRun(ctx, db.GetOpenSecretRealmRunParams{CharacterID: charID, RealmID: r.ID}); err == nil {
		if now.Before(open.ExpiresAt) {
			st.OpenRun = &OpenRunView{RunID: open.ID, ExpiresAt: open.ExpiresAt}
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return st, err
	}
	return st, nil
}

// Start issues a seeded run (or resumes the open one). Under the progression lock: idempotency (issue key) → open
// run (resume if unexpired, else close as expired/abandoned — it already counted as a run) → unlocked → runs/day
// (day row lock, counted at START so a seed can't be shopped by abandoning) → insert.
func (s *Service) Start(ctx context.Context, userID uuid.UUID, realmID, idemKey string) (StartView, error) {
	r, ok := s.Cat.Get(realmID)
	if !ok {
		return StartView{}, ErrUnknownRealm
	}
	var out StartView
	var blocked *ErrBlocked
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		ch, p, err := s.character(ctx, q, userID, true)
		if err != nil {
			return err
		}
		now := s.Now().UTC()
		stage, err := s.Project(ctx, q, userID, p)
		if err != nil {
			return err
		}
		view := func(run db.SecretRealmRun, resumed bool) error {
			st, err := s.status(ctx, q, ch.ID, stage, r, now)
			if err != nil {
				return err
			}
			out = StartView{RunID: run.ID, RealmID: run.RealmID, Seed: uint32(run.Seed), IssuedAt: run.IssuedAt, ExpiresAt: run.ExpiresAt,
				ServerNow: now, Params: json.RawMessage(run.Params), Resumed: resumed, Status: st}
			return nil
		}
		if run, err := q.GetSecretRealmRunByIssueKey(ctx, db.GetSecretRealmRunByIssueKeyParams{CharacterID: ch.ID, IssueKey: idemKey}); err == nil {
			if run.RealmID != r.ID {
				return ErrKeyReused
			}
			return view(run, true)
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if open, err := q.GetOpenSecretRealmRun(ctx, db.GetOpenSecretRealmRunParams{CharacterID: ch.ID, RealmID: r.ID}); err == nil {
			if now.Before(open.ExpiresAt) {
				return view(open, true)
			}
			if _, err := q.JudgeSecretRealmRun(ctx, db.JudgeSecretRealmRunParams{ID: open.ID, Verdict: text(VerdictExpired),
				Reason: text(ReasonAbandoned), VerdictAt: &now}); err != nil {
				return fmt.Errorf("close abandoned run: %w", err)
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		day, err := q.LockSecretRealmDay(ctx, db.LockSecretRealmDayParams{CharacterID: ch.ID, GameDay: gameday.PG(now)})
		if err != nil {
			return fmt.Errorf("lock day: %w", err)
		}
		switch {
		case stage == nil || stage.Tier < r.MinTier:
			blocked = &ErrBlocked{Code: BlockLocked}
		case int64(day.Runs) >= r.RunsPerDay:
			blocked = &ErrBlocked{Code: BlockDailyLimit}
		}
		if blocked != nil {
			blocked.Status, err = s.status(ctx, q, ch.ID, stage, r, now)
			return err // commit the abandoned-run close (if any); report the block after
		}
		if err := q.AddSecretRealmRun(ctx, db.AddSecretRealmRunParams{CharacterID: ch.ID, GameDay: gameday.PG(now)}); err != nil {
			return err
		}
		run, err := q.InsertSecretRealmRun(ctx, db.InsertSecretRealmRunParams{CharacterID: ch.ID, RealmID: r.ID, Seed: int64(s.seed()),
			Params: r.ParamsJSON(), IssueKey: idemKey, IssuedAt: now, ExpiresAt: now.Add(time.Duration(r.ExpireAfterSeconds) * time.Second),
			GameDay: gameday.PG(now)})
		if err != nil {
			return fmt.Errorf("insert run: %w", err)
		}
		return view(run, false)
	})
	if err == nil && blocked != nil {
		return StartView{}, blocked
	}
	return out, err
}

// Submit judges the run's single submission and grants its loot.
//
// The replay runs OUTSIDE the transaction (anti-cheat L5): load the run unlocked (scoped to the character), and if it
// is still unjudged, replay it against the server clock. Then, under the progression lock: run row (FOR UPDATE) →
// re-check the verdict is still empty (else same key → stored verdict + loot, Replayed; other key → 409) → band → day
// row lock (daily caps across all realms, SUBMIT day) → loot roll → verdict row → survived: điểm duyên trial_pass
// (vip_accounts) → ledgers (currency before items; key "sr:<run id>") → day totals. Lock order (000015):
// progression → secret_realm_runs → secret_realm_days → vip_accounts → currency_accounts → inventory_items.
// A replayed submit never credits điểm duyên again (M1): it reports the stored ledger row of the original credit.
func (s *Service) Submit(ctx context.Context, userID, runID uuid.UUID, idemKey string, c Claim) (SubmitView, error) {
	pre := db.New(s.Pool)
	ch, err := pre.GetCharacterByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return SubmitView{}, ErrNoCharacter
	} else if err != nil {
		return SubmitView{}, err
	}
	unlocked, err := pre.GetSecretRealmRun(ctx, db.GetSecretRealmRunParams{ID: runID, CharacterID: ch.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return SubmitView{}, ErrRunNotFound
	} else if err != nil {
		return SubmitView{}, err
	}
	r, ok := s.Cat.Get(unlocked.RealmID)
	if !ok {
		return SubmitView{}, ErrUnknownRealm
	}
	now := s.Now().UTC()
	var j Judgement
	if !unlocked.Verdict.Valid {
		j = Judge(r, Run{Seed: uint32(unlocked.Seed), Params: unlocked.Params, IssuedAt: unlocked.IssuedAt, ExpiresAt: unlocked.ExpiresAt}, c, now)
	}
	var out SubmitView
	err = postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		_, p, err := s.character(ctx, q, userID, true)
		if err != nil {
			return err
		}
		run, err := q.GetSecretRealmRunForUpdate(ctx, db.GetSecretRealmRunForUpdateParams{ID: runID, CharacterID: ch.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRunNotFound
		} else if err != nil {
			return err
		}
		replayed := false
		vipPoints := int64(0)
		if run.Verdict.Valid {
			if !run.SubmitKey.Valid || run.SubmitKey.String != idemKey {
				return ErrAlreadySubmitted
			}
			replayed = true
			if prev, err := q.GetVIPLedgerByKey(ctx, db.GetVIPLedgerByKeyParams{UserID: userID, IdempotencyKey: "sr:" + run.ID.String()}); err == nil {
				vipPoints = prev.Delta
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		} else {
			if j.Verdict == "" { // judged concurrently between the read and the lock: cannot happen without a verdict
				return ErrAlreadySubmitted
			}
			claim, _ := json.Marshal(c)
			var result []byte
			if j.Outcome != nil {
				result, _ = json.Marshal(j.Outcome)
			}
			arg := db.JudgeSecretRealmRunParams{ID: run.ID, SubmitKey: text(idemKey), SubmittedAt: &now, Claim: claim, Result: result,
				Verdict: text(j.Verdict), Reason: text(j.Reason), VerdictAt: &now}
			band := BandFor(r, run.Params, j.Verdict, j.Outcome)
			var loot *Loot
			if band != nil {
				day, err := q.LockSecretRealmDay(ctx, db.LockSecretRealmDayParams{CharacterID: ch.ID, GameDay: gameday.PG(now)})
				if err != nil {
					return fmt.Errorf("lock day: %w", err)
				}
				_, artIDs := r.ItemIDs()
				owned := map[string]bool{}
				if len(artIDs) > 0 {
					ids, err := q.OwnedItems(ctx, db.OwnedItemsParams{CharacterID: ch.ID, ItemIds: artIDs})
					if err != nil {
						return err
					}
					for _, id := range ids {
						owned[id] = true
					}
				}
				l := RollLoot(r, band, j.Verdict == VerdictPass, s.rng(), func(id string) bool { return owned[id] },
					Today{SpiritStone: day.SpiritStone, Materials: day.Materials, Artifacts: int64(day.Artifacts)})
				loot = &l
				arg.Band = text(band.ID)
				arg.Loot, _ = json.Marshal(l)
			}
			if run, err = q.JudgeSecretRealmRun(ctx, arg); err != nil {
				return fmt.Errorf("judge run: %w", err)
			}
			if j.Verdict == VerdictPass && s.VIP != nil {
				// "Qua bí cảnh hoặc mini game" (vip.yaml trial_pass): capped there + the 20/day earn cap. Credited once,
				// with the verdict (vip_accounts before currency/inventory).
				got, err := s.VIP.EarnLocked(ctx, q, userID, vip.ActTrialPass, vip.Ref{Type: "secret_realm_run", ID: run.ID.String(),
					IdemKey: "sr:" + run.ID.String()}, now)
				if err != nil {
					return fmt.Errorf("earn điểm duyên: %w", err)
				}
				vipPoints = got.Points
			}
			if loot != nil {
				if err := s.grant(ctx, q, ch.ID, run.ID, *loot); err != nil {
					return err
				}
				if err := q.AddSecretRealmRewards(ctx, db.AddSecretRealmRewardsParams{CharacterID: ch.ID, GameDay: gameday.PG(now),
					SpiritStone: loot.SpiritStone, Materials: loot.Materials(), Artifacts: int32(loot.ArtifactCount())}); err != nil {
					return err
				}
			}
		}
		out = SubmitView{RunID: run.ID, RealmID: run.RealmID, Verdict: run.Verdict.String, Reason: run.Reason.String, Replayed: replayed,
			VIPPoints: vipPoints}
		// The replay outcome (with cadence telemetry) stays in the row for audit; the client sees it only for a real game
		// result (pass/fail), never the details of a rejection.
		if len(run.Result) > 0 && (out.Verdict == VerdictPass || out.Verdict == VerdictFail) {
			var o Outcome
			if json.Unmarshal(run.Result, &o) == nil {
				out.Outcome = &o
			}
		}
		if run.Band.Valid {
			for i := range r.Bands {
				if r.Bands[i].ID == run.Band.String {
					out.Band = &r.Bands[i]
				}
			}
			var l Loot
			if json.Unmarshal(run.Loot, &l) == nil {
				out.Loot = &l
			}
		}
		stage, err := s.Project(ctx, q, userID, p)
		if err != nil {
			return err
		}
		out.Status, err = s.status(ctx, q, ch.ID, stage, r, now)
		return err
	})
	return out, err
}

// grant writes the loot through the ledgers: currency first, then items by item_id (lock order), one key per run.
func (s *Service) grant(ctx context.Context, q *db.Queries, charID, runID uuid.UUID, l Loot) error {
	key := "sr:" + runID.String()
	ref := economy.Ref{Reason: "secret_realm", Type: "secret_realm_run", ID: runID.String(), IdemKey: key}
	if l.SpiritStone > 0 {
		if _, err := economy.Credit(ctx, q, charID, economy.SpiritStone, l.SpiritStone, ref); err != nil {
			return fmt.Errorf("grant stones: %w", err)
		}
	}
	items := append([]Grant(nil), l.Items...)
	if l.Artifact != "" {
		items = append(items, Grant{ItemID: l.Artifact, Quantity: 1})
	}
	sortGrants(items)
	for _, g := range items {
		if _, err := economy.GrantItem(ctx, q, charID, g.ItemID, g.Quantity, ref); err != nil {
			return fmt.Errorf("grant %s: %w", g.ItemID, err)
		}
	}
	return nil
}

func sortGrants(g []Grant) {
	for i := 1; i < len(g); i++ {
		for j := i; j > 0 && g[j].ItemID < g[j-1].ItemID; j-- {
			g[j], g[j-1] = g[j-1], g[j]
		}
	}
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
