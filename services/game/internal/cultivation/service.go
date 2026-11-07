package cultivation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/contribution"
	"tiendao/game/internal/dailytask"
	"tiendao/game/internal/db"
	"tiendao/game/internal/fixedpoint"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/leaderboard"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/realm"
	"tiendao/game/internal/vip"
	"tiendao/game/internal/world"
)

// MinSettleSeconds: settlements closer together than this are no-ops (anti row-spam).
const MinSettleSeconds = 5

// ErrNoCharacter is returned when the user has no character yet.
var ErrNoCharacter = errors.New("cultivation: character not found")

// BucketView is one bucket's credited points today.
type BucketView struct {
	Milli    int64 `json:"milli"`
	CapMilli int64 `json:"capMilli"`
}

// CatchUpView tells the player whether the late-joiner ×2 is active.
type CatchUpView struct {
	Active       bool  `json:"active"`
	MultiplierBP int64 `json:"multiplierBp"`
}

// Today is the per-game-day points summary ("hôm nay đã tích X/150").
type Today struct {
	GameDay      string                             `json:"gameDay"`
	PointsMilli  int64                              `json:"pointsMilli"`
	CeilingMilli int64                              `json:"ceilingMilli"`
	Buckets      map[contribution.Bucket]BucketView `json:"buckets"`
	SpeedItems   BucketView                         `json:"speedItems"`
	CatchUp      CatchUpView                        `json:"catchUp"`
	ResetsAt     time.Time                          `json:"resetsAt"`
}

// Snapshot is the authoritative cultivation state of a character.
type Snapshot struct {
	StageID              string       `json:"stageId"`
	Path                 realm.PathID `json:"path"`
	Tier                 int          `json:"tier"`
	CultivationMilli     int64        `json:"cultivationMilli"`
	RequiredMilli        int64        `json:"requiredMilli"`
	CapMilli             int64        `json:"capMilli"`
	BottleneckRoomMilli  int64        `json:"bottleneckRoomMilli"`
	GrantReserveMilli    int64        `json:"grantReserveMilli"`
	IdleRateMilliPerHour int64        `json:"idleRateMilliPerHour"`
	OfflineCapSeconds    int64        `json:"offlineCapSeconds"`
	LastCultivatedAt     time.Time    `json:"lastCultivatedAt"`
	Today                Today        `json:"today"`
}

// Status is returned after a settlement.
type Status struct {
	Snapshot
	EarnedMilli    int64    `json:"earnedMilli"`    // credited by this settlement (after caps/bottleneck)
	RawMilli       int64    `json:"rawMilli"`       // idle amount before caps
	StagesAdvanced []string `json:"stagesAdvanced"` // minor stages left by auto-advance
	HitOfflineCap  bool     `json:"hitOfflineCap"`
	HitDailyCap    bool     `json:"hitDailyCap"` // bucket cap or 150/day ceiling clipped this settlement
	HitBottleneck  bool     `json:"hitBottleneck"`
	Replayed       bool     `json:"replayed"` // idempotency key already used
}

// Service owns the settlement transaction and the single credit path.
type Service struct {
	Pool         *pgxpool.Pool
	Realms       *realm.Catalogue
	Contribution *contribution.Config
	VIP          *vip.Table
	Tasks        *dailytask.Catalogue // optional: daily-task progress hooks
	Now          func() time.Time
}

// Ref identifies the cause of a credit (cultivation_credits.ref_type/ref_id) and its idempotency key.
type Ref struct {
	Type, ID, IdemKey string
}

// CreditOutcome is what CreditLocked did.
type CreditOutcome struct {
	CreditResult
	CatchUp bool
	Day     contribution.Day // today's totals after the credit
}

// WorldView is the part of world_state the credit path needs (read without lock, §2.3).
type WorldView struct {
	StartedAt   *time.Time
	MedianMilli int64
}

// World reads world_state (no lock).
func World(ctx context.Context, q *db.Queries) (WorldView, error) {
	ws, err := q.GetWorldState(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return WorldView{}, nil
	} else if err != nil {
		return WorldView{}, fmt.Errorf("world state: %w", err)
	}
	return WorldView{StartedAt: ws.StartedAt, MedianMilli: ws.MedianPositionMilli}, nil
}

func (s *Service) catchUp(w WorldView, st *realm.Stage, stored int64, now time.Time) bool {
	return s.Contribution.CatchUpEligible(w.StartedAt != nil, world.ServerDay(w.StartedAt, now), s.Realms.Position(st, stored), w.MedianMilli)
}

func dayOf(r db.DailyContribution) contribution.Day {
	return contribution.Day{Idle: r.IdleMilli, Activity: r.ActivityMilli, Community: r.CommunityMilli,
		SpeedItem: r.SpeedItemMilli, CatchUpBonus: r.CatchUpBonusMilli}
}

// readDay loads today's contribution without lock (zero when absent).
func readDay(ctx context.Context, q *db.Queries, charID uuid.UUID, now time.Time) (contribution.Day, error) {
	r, err := q.GetDailyContribution(ctx, db.GetDailyContributionParams{CharacterID: charID, GameDay: gameday.PG(now)})
	if errors.Is(err, pgx.ErrNoRows) {
		return contribution.Day{}, nil
	}
	return dayOf(r), err
}

// CreditLocked is the ONLY writer of points (C4). The caller has locked p FOR UPDATE in its transaction and
// already set p.LastCultivatedAt / p.RateCarry if time was consumed; CreditLocked persists the whole row.
// Lock order: character_progressions → daily_contribution.
func (s *Service) CreditLocked(ctx context.Context, q *db.Queries, p db.CharacterProgression, src contribution.Source,
	rawMilli int64, ref Ref, now time.Time) (db.CharacterProgression, CreditOutcome, error) {
	var out CreditOutcome
	stage, err := s.Realms.Stage(p.StageID)
	if err != nil {
		return p, out, err
	}
	gd := gameday.PG(now)
	if rawMilli > 0 {
		if err := q.EnsureDailyContribution(ctx, db.EnsureDailyContributionParams{CharacterID: p.CharacterID, GameDay: gd}); err != nil {
			return p, out, fmt.Errorf("ensure daily contribution: %w", err)
		}
		row, err := q.GetDailyContributionForUpdate(ctx, db.GetDailyContributionForUpdateParams{CharacterID: p.CharacterID, GameDay: gd})
		if err != nil {
			return p, out, fmt.Errorf("lock daily contribution: %w", err)
		}
		out.Day = dayOf(row)
		w, err := World(ctx, q)
		if err != nil {
			return p, out, err
		}
		out.CatchUp = s.catchUp(w, stage, p.CultivationMilli, now)
	} else if out.Day, err = readDay(ctx, q, p.CharacterID, now); err != nil {
		return p, out, err
	}

	out.CreditResult = Credit(s.Realms, s.Contribution, Progress{Stage: stage, StoredMilli: p.CultivationMilli, GrantReserveMilli: p.GrantReserveMilli},
		out.Day, src, rawMilli, out.CatchUp)
	a := out.Applied
	if rawMilli > 0 {
		if _, err := q.InsertCultivationCredit(ctx, db.InsertCultivationCreditParams{
			CharacterID: p.CharacterID, GameDay: gd, Source: string(src), Bucket: text(string(a.Bucket)), StageID: stage.ID,
			RawMilli: a.RawMilli, MultiplierBp: int32(a.MultiplierBP), CreditedMilli: a.CreditedMilli, CatchUpBonusMilli: a.CatchUpBonusMilli,
			ClippedCeilingMilli: a.ClippedCeilingMilli, ClippedBottleneckMilli: a.ClippedBottleneckMilli,
			RefType: text(ref.Type), RefID: text(ref.ID), IdempotencyKey: string(src) + ":" + ref.IdemKey,
		}); err != nil {
			return p, out, fmt.Errorf("insert credit: %w", err)
		}
		if a.CreditedMilli > 0 {
			add := db.AddDailyContributionParams{CharacterID: p.CharacterID, GameDay: gd, CatchUpBonusMilli: a.CatchUpBonusMilli}
			switch a.Bucket {
			case contribution.Idle:
				add.IdleMilli = a.CreditedMilli
			case contribution.Activity:
				add.ActivityMilli = a.CreditedMilli
			case contribution.Community:
				add.CommunityMilli = a.CreditedMilli
			}
			if src == contribution.SourceItem {
				add.SpeedItemMilli = a.CreditedMilli
			}
			if err := q.AddDailyContribution(ctx, add); err != nil {
				return p, out, fmt.Errorf("add daily contribution: %w", err)
			}
			out.Day = out.Day.Add(a, src)
		}
	}
	if out.GrantReleasedMilli > 0 { // not in daily_contribution: never counts for the ceiling or the world (§4.6)
		if _, err := q.InsertCultivationCredit(ctx, db.InsertCultivationCreditParams{
			CharacterID: p.CharacterID, GameDay: gd, Source: string(contribution.SourceLateJoinGrant), StageID: stage.ID,
			RawMilli: out.GrantReleasedMilli, MultiplierBp: fixedpoint.BPScale, CreditedMilli: out.GrantReleasedMilli,
			RefType: text(ref.Type), RefID: text(ref.ID), IdempotencyKey: string(contribution.SourceLateJoinGrant) + ":" + string(src) + ":" + ref.IdemKey,
		}); err != nil {
			return p, out, fmt.Errorf("insert grant release: %w", err)
		}
	}

	after := out.After
	if after.Stage != stage {
		p.StageReachedAt = now
	}
	p.RealmID, p.StageID, p.CultivationMilli, p.GrantReserveMilli = after.Stage.RealmID, after.Stage.ID, after.StoredMilli, after.GrantReserveMilli
	if err := q.UpdateProgressionCultivation(ctx, db.UpdateProgressionCultivationParams{
		CharacterID: p.CharacterID, RealmID: p.RealmID, StageID: p.StageID, CultivationMilli: p.CultivationMilli,
		RateCarry: p.RateCarry, GrantReserveMilli: p.GrantReserveMilli, LastCultivatedAt: p.LastCultivatedAt, StageReachedAt: p.StageReachedAt,
	}); err != nil {
		return p, out, fmt.Errorf("update progression: %w", err)
	}
	if err := RecordStagesReached(ctx, q, s.Realms, p.CharacterID, out.Passed, "auto", now); err != nil {
		return p, out, err
	}
	if a.CreditedMilli > 0 || out.GrantReleasedMilli > 0 {
		if err := EnqueueLeaderboard(ctx, q, p.CharacterID, leaderboard.Score(after.Stage, after.StoredMilli)); err != nil {
			return p, out, err
		}
	}
	return p, out, nil
}

// RecordStagesReached writes one progression_events row per stage reached by leaving each `passed` stage.
func RecordStagesReached(ctx context.Context, q *db.Queries, cat *realm.Catalogue, charID uuid.UUID, passed []*realm.Stage, via string, now time.Time) error {
	for _, from := range passed {
		to := cat.Next(from)
		if to == nil {
			continue
		}
		payload, _ := json.Marshal(map[string]string{"from": from.ID, "via": via})
		if err := q.InsertProgressionEvent(ctx, db.InsertProgressionEventParams{
			CharacterID: charID, Kind: "stage_reached", StageID: text(to.ID), Payload: payload, OccurredAt: now,
		}); err != nil {
			return fmt.Errorf("progression event: %w", err)
		}
	}
	return nil
}

// SettleOutcome is the result of SettleLocked.
type SettleOutcome struct {
	Now        time.Time // the server time this settlement used
	Settle     SettleResult
	OfflineCap time.Duration
	Credit     CreditOutcome
	Written    bool // false when < MinSettleSeconds elapsed (nothing written)
}

// Settle settles accumulated idle cultivation for the user's character, idempotently.
func (s *Service) Settle(ctx context.Context, userID uuid.UUID, idemKey string) (Status, error) {
	var out Status
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		ch, err := q.GetCharacterByUser(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoCharacter
		} else if err != nil {
			return err
		}
		// Lock the hot row: serializes everything that changes cultivation for this character.
		p, err := q.GetProgressionForUpdate(ctx, ch.ID)
		if err != nil {
			return fmt.Errorf("lock progression: %w", err)
		}
		// Idempotency is checked AFTER the lock so concurrent duplicates serialize here.
		if _, err := q.GetSettlementByKey(ctx, db.GetSettlementByKeyParams{CharacterID: ch.ID, IdempotencyKey: idemKey}); err == nil {
			out, err = s.statusOf(ctx, q, userID, p, SettleOutcome{}, s.Now().UTC())
			out.Replayed = true
			return err
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		p, res, err := s.SettleLocked(ctx, q, userID, p, idemKey)
		if err != nil {
			return err
		}
		if res.Written {
			if err := s.Tasks.Record(ctx, q, ch.ID, res.Now, dailytask.Settle, res.Settle.ElapsedSeconds); err != nil {
				return err
			}
			// Điểm duyên for online cultivation (decision D: 1 × 3/day, settles ≥ min_elapsed_seconds apart). Only this
			// player-initiated settle counts, not the settlements other mutations run first.
			if a, ok := s.VIP.Activity(vip.ActOnlineCultivation); ok && res.Settle.ElapsedSeconds >= a.MinElapsedSeconds {
				if _, err := s.VIP.EarnLocked(ctx, q, userID, vip.ActOnlineCultivation,
					vip.Ref{Type: "settlement", ID: idemKey, IdemKey: "settle:" + idemKey}, res.Now); err != nil {
					return fmt.Errorf("earn điểm duyên: %w", err)
				}
			}
		}
		out, err = s.statusOf(ctx, q, userID, p, res, res.Now)
		return err
	})
	return out, err
}

// SettleLocked settles idle progress for a progression row the caller has ALREADY locked FOR UPDATE, credits it
// through CreditLocked, writes the settlement log under idemKey and returns the updated row. Used by Settle and by
// other mutations (breakthrough) that need up-to-date cultivation first.
func (s *Service) SettleLocked(ctx context.Context, q *db.Queries, userID uuid.UUID, p db.CharacterProgression, idemKey string) (db.CharacterProgression, SettleOutcome, error) {
	now := s.Now().UTC()
	baseCap, offlineCap, err := s.OfflineCap(ctx, q, userID, p)
	if err != nil {
		return p, SettleOutcome{}, err
	}
	rate := s.Contribution.IdleRateMilliPerHour
	buff, err := s.Buff(ctx, q, p.CharacterID)
	if err != nil {
		return p, SettleOutcome{}, err
	}
	res := Settle(SettleInput{Now: now, LastCultivatedAt: p.LastCultivatedAt, RateCarry: int64(p.RateCarry), RateMilliPerHour: rate,
		OfflineCap: offlineCap, Buff: buff})
	out := SettleOutcome{Now: now, Settle: res, OfflineCap: offlineCap}
	// Too soon since the last settlement: no idle credit and last_cultivated_at stays put, so the elapsed time is
	// counted by the next settlement (no loss, no double award, no row spam). The late-join reserve is still released
	// so this path matches Project exactly (preview == attempt); that case writes a zero-time settlement row so the
	// idempotency key is recorded.
	if res.ElapsedSeconds < MinSettleSeconds {
		out.Settle = SettleResult{LastCultivatedAt: p.LastCultivatedAt, RateCarry: int64(p.RateCarry)}
		if p.GrantReserveMilli <= 0 {
			return p, out, nil
		}
		stage, err := s.Realms.Stage(p.StageID)
		if err != nil {
			return p, out, err
		}
		if _, rel, _ := ReleaseGrant(s.Realms, Progress{Stage: stage, StoredMilli: p.CultivationMilli, GrantReserveMilli: p.GrantReserveMilli}); rel == 0 {
			return p, out, nil
		}
		p, out.Credit, err = s.CreditLocked(ctx, q, p, contribution.SourceSettle, 0, Ref{Type: "settlement", IdemKey: idemKey}, now)
		if err != nil {
			return p, out, err
		}
		out.Written = false // not a real settlement: daily tasks don't count it
		return p, out, s.insertSettlement(ctx, q, p, idemKey, out.Settle, rate, offlineCap, out.Credit.Applied, now)
	}
	p.LastCultivatedAt, p.RateCarry = res.LastCultivatedAt, int32(res.RateCarry)
	p, out.Credit, err = s.CreditLocked(ctx, q, p, contribution.SourceSettle, res.RawMilli, Ref{Type: "settlement", IdemKey: idemKey}, now)
	if err != nil {
		return p, out, err
	}
	out.Written = true
	if err := s.insertSettlement(ctx, q, p, idemKey, res, rate, offlineCap, out.Credit.Applied, now); err != nil {
		return p, out, err
	}
	// A pending offline bonus (Tụ Linh Phù) is spent only by the settlement it actually extended.
	if p.OfflineBonusSeconds > 0 && res.ElapsedSeconds > int64(baseCap/time.Second) {
		if err := q.ClearOfflineBonus(ctx, p.CharacterID); err != nil {
			return p, out, err
		}
		p.OfflineBonusSeconds = 0
	}
	return p, out, nil
}

// insertSettlement logs a settlement. stage_id is the stage AFTER the settlement (pairs with
// cultivation_after_milli); cultivation_credits.stage_id is the stage BEFORE the credit.
// SettleLockedRow is SettleLocked returning only the updated row (injected into services that can't import this
// package, e.g. faction: settle before a membership change so offline accrual stays with the old faction).
func (s *Service) SettleLockedRow(ctx context.Context, q *db.Queries, userID uuid.UUID, p db.CharacterProgression, idemKey string) (db.CharacterProgression, error) {
	p, _, err := s.SettleLocked(ctx, q, userID, p, idemKey)
	return p, err
}

func (s *Service) insertSettlement(ctx context.Context, q *db.Queries, p db.CharacterProgression, idemKey string, res SettleResult,
	rate int64, offlineCap time.Duration, a contribution.Applied, now time.Time) error {
	mult := a.MultiplierBP
	if mult == 0 {
		mult = fixedpoint.BPScale
	}
	if _, err := q.InsertSettlement(ctx, db.InsertSettlementParams{
		CharacterID: p.CharacterID, IdempotencyKey: idemKey, StageID: p.StageID,
		ElapsedSeconds: res.ElapsedSeconds, EligibleSeconds: res.EligibleSeconds, RateMilliPerHour: rate,
		OfflineCapSeconds: int64(offlineCap / time.Second), RawMilli: res.RawMilli, MultiplierBp: int32(mult),
		CreditedMilli: a.CreditedMilli, CultivationAfterMilli: p.CultivationMilli, GameDay: gameday.PG(now), SettledAt: now,
	}); err != nil {
		return fmt.Errorf("insert settlement: %w", err)
	}
	return nil
}

// OfflineCap resolves the offline window at settlement time (documented MVP policy, GDD §5.3):
// base = VIP tier cap; effective = base extended by a pending bonus, never past the daily ceiling.
func (s *Service) OfflineCap(ctx context.Context, q *db.Queries, userID uuid.UUID, p db.CharacterProgression) (base, effective time.Duration, err error) {
	points, err := q.GetVIPPoints(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	tier := s.VIP.TierForPoints(points)
	bonus := time.Duration(p.OfflineBonusSeconds) * time.Second
	return s.VIP.OfflineCap(tier), s.VIP.EffectiveOfflineCap(tier, bonus), nil
}

// BuffSlotFortune is the character_buffs slot of the gieo quẻ buff.
const BuffSlotFortune = "fortune"

// Buff loads the character's timed rate buff (zero when none), capped by contribution idle.max_buff_bonus_bp.
// Read without lock: buffs are written only under the progression lock the settling caller holds.
func (s *Service) Buff(ctx context.Context, q *db.Queries, charID uuid.UUID) (Buff, error) {
	b, err := q.GetCharacterBuff(ctx, db.GetCharacterBuffParams{CharacterID: charID, Slot: BuffSlotFortune})
	if errors.Is(err, pgx.ErrNoRows) {
		return Buff{}, nil
	} else if err != nil {
		return Buff{}, fmt.Errorf("load buff: %w", err)
	}
	return Buff{BonusBP: min(int64(b.BonusBp), s.Contribution.MaxBuffBonusBP), Start: b.StartsAt, End: b.EndsAt}, nil
}

// Projection is what a settlement now would yield, computed WITHOUT writing (same pure functions as SettleLocked,
// so the breakthrough preview shows exactly what the attempt will apply).
type Projection struct {
	Settle     SettleResult
	OfflineCap time.Duration
	Credit     CreditResult
	CatchUp    bool
	Day        contribution.Day // today's totals including the projected credit
	Snapshot   Snapshot
}

// ProjectedStage is the stage after the pending idle settlement (read-only; same path as the breakthrough preview).
func (s *Service) ProjectedStage(ctx context.Context, q *db.Queries, userID uuid.UUID, p db.CharacterProgression) (*realm.Stage, error) {
	proj, err := s.Project(ctx, q, userID, p)
	if err != nil {
		return nil, err
	}
	return proj.Credit.After.Stage, nil
}

// Project computes the projected state for read-only views.
func (s *Service) Project(ctx context.Context, q *db.Queries, userID uuid.UUID, p db.CharacterProgression) (Projection, error) {
	stage, err := s.Realms.Stage(p.StageID)
	if err != nil {
		return Projection{}, err
	}
	now := s.Now().UTC()
	_, offlineCap, err := s.OfflineCap(ctx, q, userID, p)
	if err != nil {
		return Projection{}, err
	}
	day, err := readDay(ctx, q, p.CharacterID, now)
	if err != nil {
		return Projection{}, err
	}
	w, err := World(ctx, q)
	if err != nil {
		return Projection{}, err
	}
	buff, err := s.Buff(ctx, q, p.CharacterID)
	if err != nil {
		return Projection{}, err
	}
	pr := Projection{OfflineCap: offlineCap, CatchUp: s.catchUp(w, stage, p.CultivationMilli, now)}
	pr.Settle = Settle(SettleInput{Now: now, LastCultivatedAt: p.LastCultivatedAt, RateCarry: int64(p.RateCarry),
		RateMilliPerHour: s.Contribution.IdleRateMilliPerHour, OfflineCap: offlineCap, Buff: buff})
	raw := pr.Settle.RawMilli
	if pr.Settle.ElapsedSeconds < MinSettleSeconds {
		raw = 0
	}
	pr.Credit = Credit(s.Realms, s.Contribution, Progress{Stage: stage, StoredMilli: p.CultivationMilli, GrantReserveMilli: p.GrantReserveMilli},
		day, contribution.SourceSettle, raw, pr.CatchUp)
	pr.Day = day.Add(pr.Credit.Applied, contribution.SourceSettle)
	pr.Snapshot = s.snapshot(pr.Credit.After, pr.Day, offlineCap, pr.CatchUp, p.LastCultivatedAt, now)
	return pr, nil
}

// EnqueueLeaderboard writes a leaderboard.dirty outbox event. The worker re-reads the row, so the
// score in the payload is informational only.
func EnqueueLeaderboard(ctx context.Context, q *db.Queries, characterID uuid.UUID, score float64) error {
	payload, _ := json.Marshal(map[string]any{"characterId": characterID, "score": score})
	return q.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{Topic: "leaderboard.dirty", AggregateID: characterID.String(), Payload: payload})
}

func (s *Service) snapshot(p Progress, day contribution.Day, offlineCap time.Duration, catchUp bool, lastCultivatedAt, now time.Time) Snapshot {
	c := s.Contribution
	buckets := make(map[contribution.Bucket]BucketView, len(contribution.Buckets))
	for _, b := range contribution.Buckets {
		buckets[b] = BucketView{Milli: day.Get(b), CapMilli: c.Buckets[b].CapMilli}
	}
	mult := int64(fixedpoint.BPScale)
	if catchUp {
		mult = c.CatchUp.MultiplierBP
	}
	return Snapshot{
		StageID: p.Stage.ID, Path: p.Stage.Path, Tier: p.Stage.Tier, CultivationMilli: p.StoredMilli,
		RequiredMilli: p.Stage.RequiredMilli, CapMilli: p.Stage.CapMilli, BottleneckRoomMilli: s.Realms.BottleneckRoomMilli(p.Stage, p.StoredMilli),
		GrantReserveMilli: p.GrantReserveMilli, IdleRateMilliPerHour: c.IdleRateMilliPerHour, OfflineCapSeconds: int64(offlineCap / time.Second),
		LastCultivatedAt: lastCultivatedAt,
		Today: Today{
			GameDay: gameday.Of(now).Format(time.DateOnly), PointsMilli: day.Total(), CeilingMilli: c.DailyCeilingMilli,
			Buckets: buckets, SpeedItems: BucketView{Milli: day.SpeedItem, CapMilli: c.SpeedItemDailyMilli},
			CatchUp: CatchUpView{Active: catchUp, MultiplierBP: mult}, ResetsAt: gameday.NextReset(now).UTC(),
		},
	}
}

// SnapshotLocked builds the authoritative snapshot of a (locked or freshly written) row inside the tx.
// now must be the same server time the caller's mutation used (one clock read per request).
func (s *Service) SnapshotLocked(ctx context.Context, q *db.Queries, userID uuid.UUID, p db.CharacterProgression, now time.Time) (Snapshot, error) {
	stage, err := s.Realms.Stage(p.StageID)
	if err != nil {
		return Snapshot{}, err
	}
	_, offlineCap, err := s.OfflineCap(ctx, q, userID, p)
	if err != nil {
		return Snapshot{}, err
	}
	day, err := readDay(ctx, q, p.CharacterID, now)
	if err != nil {
		return Snapshot{}, err
	}
	w, err := World(ctx, q)
	if err != nil {
		return Snapshot{}, err
	}
	return s.snapshot(Progress{Stage: stage, StoredMilli: p.CultivationMilli, GrantReserveMilli: p.GrantReserveMilli}, day, offlineCap,
		s.catchUp(w, stage, p.CultivationMilli, now), p.LastCultivatedAt, now), nil
}

func (s *Service) statusOf(ctx context.Context, q *db.Queries, userID uuid.UUID, p db.CharacterProgression, res SettleOutcome, now time.Time) (Status, error) {
	snap, err := s.SnapshotLocked(ctx, q, userID, p, now)
	if err != nil {
		return Status{}, err
	}
	a := res.Credit.Applied
	st := Status{Snapshot: snap, EarnedMilli: a.CreditedMilli, RawMilli: res.Settle.RawMilli, StagesAdvanced: []string{},
		HitOfflineCap: res.Settle.HitOfflineCap, HitDailyCap: a.ClippedCeilingMilli > 0,
		HitBottleneck: a.ClippedBottleneckMilli > 0 || snap.BottleneckRoomMilli == 0}
	for _, ps := range res.Credit.Passed {
		st.StagesAdvanced = append(st.StagesAdvanced, ps.ID)
	}
	return st, nil
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
