package trial

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/db"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/realm"
	"tiendao/game/internal/vip"
)

var (
	ErrNoCharacter      = errors.New("trial: character not found")
	ErrUnknownKind      = errors.New("trial: unknown kind")
	ErrRunNotFound      = errors.New("trial: run not found")
	ErrAlreadySubmitted = errors.New("trial: run already submitted")
	ErrKeyReused        = errors.New("trial: idempotency key reused for another kind")
)

// Block codes of ErrBlocked (start refused). Part of the API.
const (
	BlockCooldown    = "cooldown"
	BlockPassActive  = "pass_active"
	BlockNotEligible = "not_eligible"
)

// ErrBlocked: the run cannot start now; Status says why and until when.
type ErrBlocked struct {
	Code   string
	Status KindStatus
}

func (e *ErrBlocked) Error() string { return "trial: start blocked: " + e.Code }

// Kind states in GET /trials (precedence: passed > in_progress > locked > cooldown > available).
const (
	StateAvailable  = "available"
	StateInProgress = "in_progress"
	StateCooldown   = "cooldown"
	StatePassed     = "passed"
	StateLocked     = "locked"
)

// PassView is a valid, unconsumed pass.
type PassView struct {
	RunID      uuid.UUID `json:"runId"`
	ValidUntil time.Time `json:"validUntil"`
}

// OpenRunView is the run in progress (resumable with the same seed until it expires).
type OpenRunView struct {
	RunID     uuid.UUID `json:"runId"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// KindStatus is one trial kind as seen by the character (GET /trials, and inside every mutation response).
type KindStatus struct {
	Kind                string          `json:"kind"`
	Name                string          `json:"name"`
	Summary             string          `json:"summary"`
	State               string          `json:"state"`
	Eligible            bool            `json:"eligible"`
	GateTier            int             `json:"gateTier"`
	FromRealmID         string          `json:"fromRealmId"`
	ToRealmID           string          `json:"toRealmId"`
	OpensAt             string          `json:"opensAt"`
	CooldownUntil       *time.Time      `json:"cooldownUntil"`
	Pass                *PassView       `json:"pass"`
	OpenRun             *OpenRunView    `json:"openRun"`
	LoseCooldownSeconds int64           `json:"loseCooldownSeconds"`
	PassValidSeconds    int64           `json:"passValidSeconds"`
	ExpireAfterSeconds  int64           `json:"expireAfterSeconds"`
	Params              json.RawMessage `json:"params"`
}

// StatusView is GET /trials.
type StatusView struct {
	ServerNow time.Time    `json:"serverNow"`
	Kinds     []KindStatus `json:"kinds"`
}

// StartView is POST /trials/{kind}/start.
type StartView struct {
	RunID     uuid.UUID       `json:"runId"`
	Kind      string          `json:"kind"`
	Seed      uint32          `json:"seed"`
	IssuedAt  time.Time       `json:"issuedAt"`
	ExpiresAt time.Time       `json:"expiresAt"`
	ServerNow time.Time       `json:"serverNow"`
	Params    json.RawMessage `json:"params"`
	Resumed   bool            `json:"resumed"` // the open run (same seed) or the same Idempotency-Key
	Status    KindStatus      `json:"status"`
}

// SubmitView is POST /trials/{runId}/submit: the authoritative verdict.
type SubmitView struct {
	RunID          uuid.UUID  `json:"runId"`
	Kind           string     `json:"kind"`
	Verdict        string     `json:"verdict"`
	Reason         string     `json:"reason"`
	Outcome        *Outcome   `json:"outcome"` // pass/fail only
	PassValidUntil *time.Time `json:"passValidUntil"`
	CooldownUntil  *time.Time `json:"cooldownUntil"`
	VIPPoints      int64      `json:"vipPoints"`
	Replayed       bool       `json:"replayed"`
	Status         KindStatus `json:"status"`
}

// Service owns the trial transactions.
type Service struct {
	Pool   *pgxpool.Pool
	Cat    *Catalogue
	Realms *realm.Catalogue
	// Project returns the character's stage after the pending idle settlement, read-only (cultivation.ProjectedStage).
	// Injected to keep this package free of the cultivation import (content → trial must not pull it in).
	Project func(ctx context.Context, q *db.Queries, userID uuid.UUID, p db.CharacterProgression) (*realm.Stage, error)
	VIP     *vip.Table // nil → no điểm duyên for a pass
	Now     func() time.Time
	Seed    func() uint32 // nil → crypto/rand
}

func (s *Service) realms() *realm.Catalogue { return s.Realms }

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

// Status lists every kind for the character (read-only).
func (s *Service) Status(ctx context.Context, userID uuid.UUID) (StatusView, error) {
	q := db.New(s.Pool)
	ch, p, err := s.character(ctx, q, userID, false)
	if err != nil {
		return StatusView{}, err
	}
	stage, err := s.stage(ctx, q, userID, p)
	if err != nil {
		return StatusView{}, err
	}
	now := s.Now().UTC()
	out := StatusView{ServerNow: now, Kinds: make([]KindStatus, 0, len(s.Cat.All()))}
	for _, k := range s.Cat.All() {
		st, err := s.status(ctx, q, ch.ID, stage, k, now)
		if err != nil {
			return StatusView{}, err
		}
		out.Kinds = append(out.Kinds, st)
	}
	return out, nil
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

// stage is the character's stage after the pending idle settlement (same projection as the breakthrough preview),
// so a player whose offline time already reached Viên Mãn can start without settling first.
func (s *Service) stage(ctx context.Context, q *db.Queries, userID uuid.UUID, p db.CharacterProgression) (*realm.Stage, error) {
	return s.Project(ctx, q, userID, p)
}

// Eligible reports whether a character at stage may start kind k.
func Eligible(k *Kind, stage *realm.Stage) bool {
	if stage == nil || stage.Tier != k.Gate.Tier {
		return false
	}
	return k.OpensAt == OpensAtRealm || stage.IsLastOfRealm()
}

func (s *Service) status(ctx context.Context, q *db.Queries, charID uuid.UUID, stage *realm.Stage, k *Kind, now time.Time) (KindStatus, error) {
	st := KindStatus{Kind: k.ID, Name: k.Name, Summary: k.Summary, GateTier: k.Gate.Tier, OpensAt: k.OpensAt,
		LoseCooldownSeconds: k.LoseCooldownSeconds, PassValidSeconds: k.PassValidSeconds, ExpireAfterSeconds: k.ExpireAfterSeconds,
		Params: k.ParamsJSON(), Eligible: Eligible(k, stage)}
	if from, ok := s.realms().StageAt(stage.Path, k.Gate.Tier, 1); ok {
		st.FromRealmID = from.RealmID
	}
	if to, ok := s.realms().StageAt(stage.Path, k.Gate.Tier+1, 1); ok {
		st.ToRealmID = to.RealmID
	}
	if pass, err := ValidPass(ctx, q, charID, k.ID, now, false); err != nil {
		return st, err
	} else if pass != nil {
		st.Pass = &PassView{RunID: pass.ID, ValidUntil: *pass.PassValidUntil}
	}
	cd, err := q.GetTrialCooldown(ctx, db.GetTrialCooldownParams{CharacterID: charID, Kind: k.ID})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return st, err
	}
	if open, err := q.GetOpenTrialRun(ctx, db.GetOpenTrialRunParams{CharacterID: charID, Kind: k.ID}); err == nil {
		if now.Before(open.ExpiresAt) {
			st.OpenRun = &OpenRunView{RunID: open.ID, ExpiresAt: open.ExpiresAt}
		} else if until := abandonCooldown(k, open); cd == nil || until.After(*cd) {
			cd = &until // closed as abandoned on the next start; shown already
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return st, err
	}
	if cd != nil && now.Before(*cd) {
		st.CooldownUntil = cd
	}
	switch {
	case st.Pass != nil:
		st.State = StatePassed
	case st.OpenRun != nil:
		st.State = StateInProgress
	case !st.Eligible:
		st.State = StateLocked
	case st.CooldownUntil != nil:
		st.State = StateCooldown
	default:
		st.State = StateAvailable
	}
	return st, nil
}

// abandonCooldown: a run never submitted counts as a loss from the moment it expired (no seed shopping).
func abandonCooldown(k *Kind, r db.TrialRun) time.Time {
	return r.ExpiresAt.Add(time.Duration(k.LoseCooldownSeconds) * time.Second)
}

// Start issues a seeded run (or resumes the open one). Under the progression lock: idempotency (issue key) →
// open run (resume if unexpired, else close as expired/abandoned with a cooldown) → cooldown → valid pass →
// eligibility → insert.
func (s *Service) Start(ctx context.Context, userID uuid.UUID, kindID, idemKey string) (StartView, error) {
	k, ok := s.Cat.Get(kindID)
	if !ok {
		return StartView{}, ErrUnknownKind
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
		stage, err := s.stage(ctx, q, userID, p)
		if err != nil {
			return err
		}
		view := func(r db.TrialRun, resumed bool) error {
			st, err := s.status(ctx, q, ch.ID, stage, k, now)
			if err != nil {
				return err
			}
			out = StartView{RunID: r.ID, Kind: r.Kind, Seed: uint32(r.Seed), IssuedAt: r.IssuedAt, ExpiresAt: r.ExpiresAt, ServerNow: now,
				Params: json.RawMessage(r.Params), Resumed: resumed, Status: st}
			return nil
		}
		if r, err := q.GetTrialRunByIssueKey(ctx, db.GetTrialRunByIssueKeyParams{CharacterID: ch.ID, IssueKey: idemKey}); err == nil {
			if r.Kind != k.ID {
				return ErrKeyReused
			}
			return view(r, true)
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if open, err := q.GetOpenTrialRun(ctx, db.GetOpenTrialRunParams{CharacterID: ch.ID, Kind: k.ID}); err == nil {
			if now.Before(open.ExpiresAt) {
				return view(open, true)
			}
			cd := abandonCooldown(k, open)
			if _, err := q.JudgeTrialRun(ctx, db.JudgeTrialRunParams{ID: open.ID, Verdict: text(VerdictExpired), Reason: text(ReasonAbandoned),
				VerdictAt: &now, CooldownUntil: &cd}); err != nil {
				return fmt.Errorf("close abandoned run: %w", err)
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		st, err := s.status(ctx, q, ch.ID, stage, k, now)
		if err != nil {
			return err
		}
		switch {
		case st.Pass != nil:
			blocked = &ErrBlocked{Code: BlockPassActive, Status: st}
		case !st.Eligible:
			blocked = &ErrBlocked{Code: BlockNotEligible, Status: st}
		case st.CooldownUntil != nil:
			blocked = &ErrBlocked{Code: BlockCooldown, Status: st}
		}
		if blocked != nil {
			return nil // commit the abandoned-run close (if any); report the block after
		}
		r, err := q.InsertTrialRun(ctx, db.InsertTrialRunParams{CharacterID: ch.ID, Kind: k.ID, Seed: int64(s.seed()), Params: k.ParamsJSON(),
			IssueKey: idemKey, IssuedAt: now, ExpiresAt: now.Add(time.Duration(k.ExpireAfterSeconds) * time.Second)})
		if err != nil {
			return fmt.Errorf("insert run: %w", err)
		}
		return view(r, false)
	})
	if err == nil && blocked != nil {
		return StartView{}, blocked
	}
	return out, err
}

// Submit judges the run's single submission. Under the progression lock: run row (FOR UPDATE, scoped to the
// character) → already judged? (same key → stored verdict, Replayed; other key → ErrAlreadySubmitted) → Judge
// (server clock, replay) → write verdict (+cooldown or pass validity) → pass: điểm duyên trial_pass (capped by vip.yaml).
func (s *Service) Submit(ctx context.Context, userID, runID uuid.UUID, idemKey string, c Claim) (SubmitView, error) {
	var out SubmitView
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		ch, p, err := s.character(ctx, q, userID, true)
		if err != nil {
			return err
		}
		r, err := q.GetTrialRunForUpdate(ctx, db.GetTrialRunForUpdateParams{ID: runID, CharacterID: ch.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRunNotFound
		} else if err != nil {
			return err
		}
		k, ok := s.Cat.Get(r.Kind)
		if !ok {
			return ErrUnknownKind
		}
		now := s.Now().UTC()
		replayed := false
		if r.Verdict.Valid {
			if !r.SubmitKey.Valid || r.SubmitKey.String != idemKey {
				return ErrAlreadySubmitted
			}
			replayed = true
		} else {
			j := Judge(k, Run{Seed: uint32(r.Seed), Params: r.Params, IssuedAt: r.IssuedAt, ExpiresAt: r.ExpiresAt}, c, now)
			claim, _ := json.Marshal(c)
			var result []byte
			if j.Outcome != nil {
				result, _ = json.Marshal(j.Outcome)
			}
			arg := db.JudgeTrialRunParams{ID: r.ID, SubmitKey: text(idemKey), SubmittedAt: &now, Claim: claim, Result: result,
				Verdict: text(j.Verdict), Reason: text(j.Reason), VerdictAt: &now}
			if j.Verdict == VerdictPass {
				until := now.Add(time.Duration(k.PassValidSeconds) * time.Second)
				arg.PassValidUntil = &until
			} else {
				until := now.Add(time.Duration(k.LoseCooldownSeconds) * time.Second)
				arg.CooldownUntil = &until
			}
			if r, err = q.JudgeTrialRun(ctx, arg); err != nil {
				return fmt.Errorf("judge run: %w", err)
			}
		}
		out = SubmitView{RunID: r.ID, Kind: r.Kind, Verdict: r.Verdict.String, Reason: r.Reason.String, PassValidUntil: r.PassValidUntil,
			CooldownUntil: r.CooldownUntil, Replayed: replayed}
		// The replay outcome (with cadence telemetry) stays in trial_runs.result for audit; the client sees it only for
		// a real game result (pass/fail), never the details of a rejection.
		if len(r.Result) > 0 && (out.Verdict == VerdictPass || out.Verdict == VerdictFail) {
			var o Outcome
			if err := json.Unmarshal(r.Result, &o); err == nil {
				out.Outcome = &o
			}
		}
		if out.Verdict == VerdictPass && s.VIP != nil && !replayed {
			// Once per pass verdict (the verdict is written once); capped by vip.yaml trial_pass + the daily earn cap.
			got, err := s.VIP.EarnLocked(ctx, q, userID, vip.ActTrialPass, vip.Ref{Type: "trial_run", ID: r.ID.String(),
				IdemKey: "trial:" + r.ID.String()}, now)
			if err != nil {
				return fmt.Errorf("earn điểm duyên: %w", err)
			}
			out.VIPPoints = got.Points
		}
		stage, err := s.stage(ctx, q, userID, p)
		if err != nil {
			return err
		}
		out.Status, err = s.status(ctx, q, ch.ID, stage, k, now)
		return err
	})
	return out, err
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
