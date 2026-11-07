package world

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"tiendao/game/internal/db"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/platform/postgres"
)

// Outbox topics written by the world tick (published as WS envelopes of the same type by the worker drain).
const (
	TopicMilestoneReached = "world.milestone_reached"
	TopicEventEnded       = "world.event_ended"
)

// MaxEventsEndedPerTick bounds the event-end work of one tick (the rest follow on the next ticks).
const MaxEventsEndedPerTick = 16

const day = 24 * time.Hour

// MilestoneReachedEvent is the outbox payload of world.milestone_reached.
type MilestoneReachedEvent struct {
	MilestoneID string    `json:"milestoneId"`
	Name        string    `json:"name"`
	Seq         int       `json:"seq"`
	Reason      Reason    `json:"reason"`
	ServerDay   int       `json:"serverDay"`
	EraID       string    `json:"eraId,omitempty"`
	EraName     string    `json:"eraName,omitempty"`
	Announce    string    `json:"announce"`
	ReachedAt   time.Time `json:"reachedAt"`
}

// EventEndedEvent is the outbox payload of world.event_ended.
type EventEndedEvent struct {
	EventID     string    `json:"eventId"`
	MilestoneID string    `json:"milestoneId"`
	ServerDay   int       `json:"serverDay"`
	EraID       string    `json:"eraId,omitempty"`
	EraName     string    `json:"eraName,omitempty"`
	Announce    string    `json:"announce"`
	EndedAt     time.Time `json:"endedAt"`
}

func (s *Service) log() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

// setEra moves the era forward only: an era earlier in catalogue order than the current one is ignored (a ranged
// event's on_end may fire after a later milestone already advanced the era, decision H allows no minimum gap).
// Moving forward schedules the era-start effects of every era it reaches or skips over (EraEffects) as world_effects
// rows of milestoneID — the milestone that moved the era, or the one whose event's on_end did. The era only moves
// forward, under the world lock, so each era's effects are scheduled exactly once (ON CONFLICT DO NOTHING besides).
func (s *Service) setEra(ctx context.Context, q *db.Queries, ws *db.WorldState, eraID, milestoneID string, now time.Time) error {
	target, ok := s.Cat.Era(eraID)
	if !ok {
		return fmt.Errorf("unknown era %q", eraID)
	}
	from := 0 // NULL era (before launch) = nothing reached yet: the first era's effects run too
	if cur, ok := s.Cat.Era(ws.EraID.String); ws.EraID.Valid && ok {
		if cur.Seq >= target.Seq {
			return nil
		}
		from = cur.Seq + 1
	}
	ws.EraID = pgtype.Text{String: eraID, Valid: true}
	ws.EraStartedAt = &now
	if err := q.SetWorldEra(ctx, db.SetWorldEraParams{EraID: ws.EraID, EraStartedAt: &now}); err != nil {
		return err
	}
	for seq := from; seq <= target.Seq; seq++ {
		for _, ee := range s.Cat.EraEffects(seq) {
			if err := insertPendingEffect(ctx, q, milestoneID, EraEffectIndexBase+ee.Index, ee.Effect, now); err != nil {
				return fmt.Errorf("era %s effect %d: %w", s.Cat.Eras[seq].ID, ee.Index, err)
			}
			s.log().Info("era-start effect scheduled", "era", s.Cat.Eras[seq].ID, "kind", ee.Effect.Kind, "from", ee.Effect.From,
				"into", ee.Effect.Into, "milestoneId", milestoneID)
		}
	}
	return nil
}

// insertPendingEffect stores a faction effect (split_faction / migrate_faction) as a pending world_effects row:
// due_at = now + notice_days, and for a split deadline_at = due_at + choose_days. faction.Effects executes it.
func insertPendingEffect(ctx context.Context, q *db.Queries, milestoneID string, index int, e Effect, now time.Time) error {
	params, _ := json.Marshal(e)
	p := db.InsertWorldEffectParams{MilestoneID: milestoneID, EffectIndex: int16(index), Kind: string(e.Kind), Snapshot: params,
		DueAt: now.Add(time.Duration(e.NoticeDays) * day)}
	switch e.Kind {
	case EffectSplitFaction:
		deadline := p.DueAt.Add(time.Duration(e.ChooseDays) * day)
		p.DeadlineAt = &deadline
	case EffectMigrateFaction:
	default:
		return fmt.Errorf("effect kind %q is not a pending faction effect", e.Kind)
	}
	return q.InsertWorldEffect(ctx, p)
}

func (s *Service) eraName(ws *db.WorldState) string {
	if e, ok := s.Cat.Era(ws.EraID.String); ok {
		return e.Name
	}
	return ""
}

// endEvents ends due ranged events (bounded), applies their on_end effects and writes world.event_ended.
func (s *Service) endEvents(ctx context.Context, q *db.Queries, ws *db.WorldState, now time.Time) (int, error) {
	due, err := q.ListDueWorldEvents(ctx, db.ListDueWorldEventsParams{Now: now, Lim: MaxEventsEndedPerTick})
	if err != nil {
		return 0, fmt.Errorf("list due events: %w", err)
	}
	n := 0
	for _, ev := range due {
		rows, err := q.EndWorldEvent(ctx, db.EndWorldEventParams{EndedAt: &now, EventID: ev.EventID})
		if err != nil {
			return n, fmt.Errorf("end event %s: %w", ev.EventID, err)
		}
		if rows == 0 {
			continue
		}
		n++
		if _, eff, ok := s.Cat.Event(ev.EventID); ok {
			for _, oe := range eff.OnEnd {
				if oe.Kind == EffectSetEra {
					if err := s.setEra(ctx, q, ws, oe.EraID, ev.MilestoneID, now); err != nil {
						return n, err
					}
				}
			}
		} else {
			s.log().Warn("world event not in catalogue; ended without on_end", "eventId", ev.EventID)
		}
		payload, _ := json.Marshal(EventEndedEvent{EventID: ev.EventID, MilestoneID: ev.MilestoneID, ServerDay: ServerDay(ws.StartedAt, now),
			EraID: ws.EraID.String, EraName: s.eraName(ws), Announce: EventEndKey(ev.EventID), EndedAt: now})
		if err := q.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{Topic: TopicEventEnded, AggregateID: ev.EventID, Payload: payload}); err != nil {
			return n, err
		}
	}
	return n, nil
}

// snapshot loads what the reach rule needs (reached set + overrides) for ws.
func snapshot(ctx context.Context, q *db.Queries, ws db.WorldState) (Snapshot, []db.WorldMilestonesReached, map[string]db.WorldMilestoneOverride, error) {
	reachedRows, err := q.ListMilestonesReached(ctx)
	if err != nil {
		return Snapshot{}, nil, nil, fmt.Errorf("list reached: %w", err)
	}
	ovRows, err := q.ListMilestoneOverrides(ctx)
	if err != nil {
		return Snapshot{}, nil, nil, fmt.Errorf("list overrides: %w", err)
	}
	snap := Snapshot{StartedAt: ws.StartedAt, ProgressMilli: ws.ProgressMilli, Paused: ws.PausedAt != nil,
		Reached: map[string]bool{}, Overrides: map[string]Override{}}
	for _, r := range reachedRows {
		snap.Reached[r.MilestoneID] = true
	}
	byID := map[string]db.WorldMilestoneOverride{}
	for _, o := range ovRows {
		byID[o.MilestoneID] = o
		snap.Overrides[o.MilestoneID] = overrideOf(o)
	}
	return snap, reachedRows, byID, nil
}

func overrideOf(o db.WorldMilestoneOverride) Override {
	ov := Override{Force: o.ForceReach, Paused: o.Paused}
	if o.ThresholdPointsPerActive.Valid {
		v := o.ThresholdPointsPerActive.Int64 * 1000
		ov.ThresholdMilli = &v
	}
	if o.TargetDay.Valid {
		v := int(o.TargetDay.Int32)
		ov.TargetDay = &v
	}
	return ov
}

// reach evaluates Next and records at most one milestone with its effects and outbox row.
func (s *Service) reach(ctx context.Context, q *db.Queries, ws *db.WorldState, now time.Time) (string, error) {
	snap, _, ovRows, err := snapshot(ctx, q, *ws)
	if err != nil {
		return "", err
	}
	d, ok := s.Cat.Next(snap, now)
	if !ok {
		return "", nil
	}
	m := d.Milestone
	var admin pgtype.UUID
	if d.Reason == ReasonAdmin {
		admin = pgtype.UUID{Bytes: ovRows[m.ID].UpdatedBy, Valid: true}
	}
	serverDay := ServerDay(ws.StartedAt, now)
	_, err = q.InsertMilestoneReached(ctx, db.InsertMilestoneReachedParams{MilestoneID: m.ID, Seq: int32(m.Seq), ReachedAt: now,
		Reason: string(d.Reason), ServerDay: int32(serverDay), ProgressMilli: ws.ProgressMilli, ThresholdMilli: d.ThresholdMilli,
		TargetDay: int32(d.TargetDay), AdminUserID: admin})
	if errors.Is(err, pgx.ErrNoRows) {
		// Next only returns unreached milestones and the tick holds the advisory lock, so a conflict here means the
		// seq is taken by ANOTHER milestone id: the catalogue was reordered after launch (milestones are append-only
		// once reached). Nothing is recorded; an operator must fix the catalogue.
		s.log().Error("milestone not recorded: seq conflict (catalogue reordered after launch?)", "milestoneId", m.ID, "seq", m.Seq)
		return "", nil
	} else if err != nil {
		return "", fmt.Errorf("insert reached %s: %w", m.ID, err)
	}
	for i, e := range m.Effects {
		if e.Kind == EffectSplitFaction || e.Kind == EffectMigrateFaction {
			// Pending: executed by faction.Effects (worker). Notice now → move at due_at (split: defaults after deadline_at).
			if err := insertPendingEffect(ctx, q, m.ID, i, e, now); err != nil {
				return "", fmt.Errorf("insert effect %s[%d]: %w", m.ID, i, err)
			}
			continue
		}
		params, _ := json.Marshal(e)
		p := db.InsertWorldEffectParams{MilestoneID: m.ID, EffectIndex: int16(i), Kind: string(e.Kind), DueAt: now, Snapshot: params, AppliedAt: &now}
		switch e.Kind {
		case EffectSetEra:
			if err := s.setEra(ctx, q, ws, e.EraID, m.ID, now); err != nil {
				return "", err
			}
		case EffectStartEvent:
			if err := q.InsertWorldEvent(ctx, db.InsertWorldEventParams{EventID: e.EventID, MilestoneID: m.ID, StartedAt: now,
				EndsAt: now.Add(time.Duration(e.DurationDays) * day)}); err != nil {
				return "", fmt.Errorf("start event %s: %w", e.EventID, err)
			}
		}
		if err := q.InsertWorldEffect(ctx, p); err != nil {
			return "", fmt.Errorf("insert effect %s[%d]: %w", m.ID, i, err)
		}
	}
	if err := q.SetWorldLastMilestone(ctx, pgtype.Text{String: m.ID, Valid: true}); err != nil {
		return "", err
	}
	payload, _ := json.Marshal(MilestoneReachedEvent{MilestoneID: m.ID, Name: m.Name, Seq: m.Seq, Reason: d.Reason, ServerDay: serverDay,
		EraID: ws.EraID.String, EraName: s.eraName(ws), Announce: m.Announce, ReachedAt: now})
	if err := q.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{Topic: TopicMilestoneReached, AggregateID: m.ID, Payload: payload}); err != nil {
		return "", err
	}
	s.log().Info("world milestone reached", "milestoneId", m.ID, "reason", d.Reason, "serverDay", serverDay, "progressMilli", ws.ProgressMilli)
	return m.ID, nil
}

// ───────────────────────────── Read model: GET /world ─────────────────────────────

// EraView names an era.
type EraView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ProgressView is world progress in milli-points per active player.
type ProgressView struct {
	PerActiveMilli        int64  `json:"perActiveMilli"`
	ClosedThroughDay      string `json:"closedThroughDay,omitempty"`
	TodayProvisionalMilli int64  `json:"todayProvisionalMilli"`
	YesterdayDAU          int32  `json:"yesterdayDau"`
	ReferencePerDayMilli  int64  `json:"referencePerDayMilli"`
}

// ReachedView is a reached milestone.
type ReachedView struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ReachedAt time.Time `json:"reachedAt"`
	ServerDay int       `json:"serverDay"`
	Reason    Reason    `json:"reason"`
}

// NextView is the next milestone with its window and estimate (override applied).
type NextView struct {
	ID                      string     `json:"id"`
	Name                    string     `json:"name"`
	TargetDay               int        `json:"targetDay"`
	ThresholdPerActiveMilli int64      `json:"thresholdPerActiveMilli"`
	ProgressBP              int64      `json:"progressBp"`
	EarliestAt              *time.Time `json:"earliestAt"`
	LatestAt                *time.Time `json:"latestAt"`
	EstimatedAt             *time.Time `json:"estimatedAt"`
}

// EventView is an open ranged event.
type EventView struct {
	ID          string    `json:"id"`
	MilestoneID string    `json:"milestoneId"`
	Name        string    `json:"name"`
	StartedAt   time.Time `json:"startedAt"`
	EndsAt      time.Time `json:"endsAt"`
}

// EffectView is a scheduled faction effect not applied yet.
type EffectView struct {
	Kind        EffectKind `json:"kind"`
	MilestoneID string     `json:"milestoneId"`
	DueAt       time.Time  `json:"dueAt"`
	DeadlineAt  *time.Time `json:"deadlineAt,omitempty"`
	From        string     `json:"from,omitempty"`
	Into        []string   `json:"into,omitempty"`
	To          string     `json:"to,omitempty"`
}

// TimelineEntry is one milestone of the catalogue with its reached state.
type TimelineEntry struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	TargetDay  int        `json:"targetDay"`
	EventDays  int        `json:"eventDays"` // duration of its ranged event, 0 when none
	Reached    bool       `json:"reached"`
	ReachedAt  *time.Time `json:"reachedAt"`
	ReachedDay *int       `json:"reachedDay"`
}

// View is GET /world (§11).
type View struct {
	Started        bool            `json:"started"`
	StartedAt      *time.Time      `json:"startedAt"`
	ServerDay      int             `json:"serverDay"`
	Paused         bool            `json:"paused"`
	Era            *EraView        `json:"era"`
	Progress       ProgressView    `json:"progress"`
	LastMilestone  *ReachedView    `json:"lastMilestone"`
	NextMilestone  *NextView       `json:"nextMilestone"`
	ActiveEvents   []EventView     `json:"activeEvents"`
	PendingEffects []EffectView    `json:"pendingEffects"`
	Timeline       []TimelineEntry `json:"timeline"`
}

// paceDays is how many recent closed days feed estimatedAt (§11: avg of the last 7 closed per_capita_milli).
const paceDays = 7

// View builds GET /world from a consistent read (no locks; REPEATABLE READ snapshot).
func (s *Service) View(ctx context.Context) (View, error) {
	var v View
	err := pgx.BeginTxFunc(ctx, s.Pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		var err error
		v, _, err = s.view(ctx, db.New(tx), s.Now().UTC())
		return err
	})
	return v, err
}

func (s *Service) view(ctx context.Context, q *db.Queries, now time.Time) (View, []db.WorldDailyTotal, error) {
	ws, err := q.GetWorldState(ctx)
	if err != nil {
		return View{}, nil, fmt.Errorf("world state: %w", err)
	}
	snap, reached, _, err := snapshot(ctx, q, ws)
	if err != nil {
		return View{}, nil, err
	}
	totals, err := q.ListRecentWorldTotals(ctx, 14)
	if err != nil {
		return View{}, nil, err
	}
	events, err := q.ListOpenWorldEvents(ctx)
	if err != nil {
		return View{}, nil, err
	}
	effects, err := q.ListPendingWorldEffects(ctx, 50)
	if err != nil {
		return View{}, nil, err
	}
	v := View{Started: ws.StartedAt != nil, StartedAt: ws.StartedAt, ServerDay: ServerDay(ws.StartedAt, now), Paused: ws.PausedAt != nil,
		ActiveEvents: []EventView{}, PendingEffects: []EffectView{}, Timeline: make([]TimelineEntry, 0, len(s.Cat.Milestones))}
	if e, ok := s.Cat.Era(ws.EraID.String); ok {
		v.Era = &EraView{ID: e.ID, Name: e.Name}
	} else if v.Started && len(s.Cat.Eras) > 0 {
		v.Era = &EraView{ID: s.Cat.Eras[0].ID, Name: s.Cat.Eras[0].Name} // NULL = first era (§5.4)
	}

	v.Progress = ProgressView{PerActiveMilli: ws.ProgressMilli, ReferencePerDayMilli: s.Cat.Settings.ReferencePerActiveMilli}
	if ws.ClosedThroughDay.Valid {
		v.Progress.ClosedThroughDay = ws.ClosedThroughDay.Time.Format(time.DateOnly)
	}
	today := gameday.Of(now)
	var paceSum, paceN int64
	for _, t := range totals {
		switch {
		case t.GameDay.Time.Equal(today) && t.ClosedAt == nil:
			v.Progress.TodayProvisionalMilli = t.PerCapitaMilli
		case t.GameDay.Time.Equal(today.AddDate(0, 0, -1)):
			v.Progress.YesterdayDAU = t.Dau
		}
		if t.ClosedAt != nil && paceN < paceDays && ws.StartedAt != nil && !t.GameDay.Time.Before(gameday.Of(*ws.StartedAt)) {
			paceSum += t.PerCapitaMilli
			paceN++
		}
	}

	reachedAt := map[string]db.WorldMilestonesReached{}
	for _, r := range reached {
		reachedAt[r.MilestoneID] = r
	}
	for _, m := range s.Cat.Milestones {
		te := TimelineEntry{ID: m.ID, Name: m.Name, TargetDay: m.TargetDay}
		if ev, ok := m.Event(); ok {
			te.EventDays = ev.DurationDays
		}
		if r, ok := reachedAt[m.ID]; ok {
			at, d := r.ReachedAt, int(r.ServerDay)
			te.Reached, te.ReachedAt, te.ReachedDay = true, &at, &d
		}
		v.Timeline = append(v.Timeline, te)
	}
	if n := len(reached); n > 0 {
		r := reached[n-1]
		name := r.MilestoneID
		if m, ok := s.Cat.Milestone(r.MilestoneID); ok {
			name = m.Name
		}
		v.LastMilestone = &ReachedView{ID: r.MilestoneID, Name: name, ReachedAt: r.ReachedAt, ServerDay: int(r.ServerDay), Reason: Reason(r.Reason)}
	}
	if m := s.Cat.NextUnreached(snap.Reached); m != nil {
		thr, td := Effective(m, snap.Overrides[m.ID])
		nv := &NextView{ID: m.ID, Name: m.Name, TargetDay: td, ThresholdPerActiveMilli: thr, ProgressBP: ProgressBP(ws.ProgressMilli, thr)}
		if ws.StartedAt != nil {
			e, l := s.Cat.Window(td, *ws.StartedAt)
			nv.EarliestAt, nv.LatestAt = &e, &l
			var avg int64
			if paceN > 0 {
				avg = paceSum / paceN
			}
			nv.EstimatedAt = Estimate(now, ws.ProgressMilli, thr, avg, e, l)
		}
		v.NextMilestone = nv
	}
	for _, e := range events {
		name := e.EventID
		if m, _, ok := s.Cat.Event(e.EventID); ok {
			name = m.Name
		}
		v.ActiveEvents = append(v.ActiveEvents, EventView{ID: e.EventID, MilestoneID: e.MilestoneID, Name: name, StartedAt: e.StartedAt, EndsAt: e.EndsAt})
	}
	for _, e := range effects {
		var eff Effect
		_ = json.Unmarshal(e.Snapshot, &eff)
		v.PendingEffects = append(v.PendingEffects, EffectView{Kind: EffectKind(e.Kind), MilestoneID: e.MilestoneID, DueAt: e.DueAt,
			DeadlineAt: e.DeadlineAt, From: eff.From, Into: eff.Into, To: eff.To})
	}
	return v, totals, nil
}

// ───────────────────────────── Admin ─────────────────────────────

// Admin errors (mapped to HTTP in httpapi).
var (
	ErrUnknownMilestone = errors.New("world: unknown milestone")
	ErrMilestoneReached = errors.New("world: milestone already reached")
	ErrInvalidOverride  = errors.New("world: invalid override")
)

// DayTotal is one world_daily_totals row for admins.
type DayTotal struct {
	GameDay             string     `json:"gameDay"`
	CreditedMilli       int64      `json:"creditedMilli"`
	CatchUpBonusMilli   int64      `json:"catchUpBonusMilli"`
	CountableMilli      int64      `json:"countableMilli"`
	DAU                 int32      `json:"dau"`
	PerCapitaMilli      int64      `json:"perCapitaMilli"`
	MedianPositionMilli *int64     `json:"medianPositionMilli"`
	ClosedAt            *time.Time `json:"closedAt"`
}

// OverrideView is one admin override.
type OverrideView struct {
	MilestoneID              string    `json:"milestoneId"`
	ThresholdPointsPerActive *int64    `json:"thresholdPointsPerActive"`
	TargetDay                *int      `json:"targetDay"`
	ForceReach               bool      `json:"forceReach"`
	Paused                   bool      `json:"paused"`
	UpdatedBy                uuid.UUID `json:"updatedBy"`
	UpdatedAt                time.Time `json:"updatedAt"`
}

func overrideView(o db.WorldMilestoneOverride) OverrideView {
	ov := OverrideView{MilestoneID: o.MilestoneID, ForceReach: o.ForceReach, Paused: o.Paused, UpdatedBy: o.UpdatedBy, UpdatedAt: o.UpdatedAt}
	if o.ThresholdPointsPerActive.Valid {
		v := o.ThresholdPointsPerActive.Int64
		ov.ThresholdPointsPerActive = &v
	}
	if o.TargetDay.Valid {
		v := int(o.TargetDay.Int32)
		ov.TargetDay = &v
	}
	return ov
}

// AdminView is GET /admin/world: the public view + raw state, last 14 day totals, overrides and reached rows.
type AdminView struct {
	World     View           `json:"world"`
	State     State          `json:"state"`
	Totals    []DayTotal     `json:"totals"`
	Overrides []OverrideView `json:"overrides"`
	Reached   []ReachedView  `json:"reached"`
}

// AdminView reads everything an operator needs to steer the story clock.
func (s *Service) AdminView(ctx context.Context) (AdminView, error) {
	var out AdminView
	err := pgx.BeginTxFunc(ctx, s.Pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		q := db.New(tx)
		now := s.Now().UTC()
		v, totals, err := s.view(ctx, q, now)
		if err != nil {
			return err
		}
		ws, err := q.GetWorldState(ctx)
		if err != nil {
			return err
		}
		ovs, err := q.ListMilestoneOverrides(ctx)
		if err != nil {
			return err
		}
		reached, err := q.ListMilestonesReached(ctx)
		if err != nil {
			return err
		}
		out = AdminView{World: v, State: stateOf(ws, now), Totals: make([]DayTotal, 0, len(totals)), Overrides: make([]OverrideView, 0, len(ovs)),
			Reached: make([]ReachedView, 0, len(reached))}
		for _, t := range totals {
			dt := DayTotal{GameDay: t.GameDay.Time.Format(time.DateOnly), CreditedMilli: t.CreditedMilli, CatchUpBonusMilli: t.CatchUpBonusMilli,
				CountableMilli: t.CountableMilli, DAU: t.Dau, PerCapitaMilli: t.PerCapitaMilli, ClosedAt: t.ClosedAt}
			if t.MedianPositionMilli.Valid {
				m := t.MedianPositionMilli.Int64
				dt.MedianPositionMilli = &m
			}
			out.Totals = append(out.Totals, dt)
		}
		for _, o := range ovs {
			out.Overrides = append(out.Overrides, overrideView(o))
		}
		for _, r := range reached {
			name := r.MilestoneID
			if m, ok := s.Cat.Milestone(r.MilestoneID); ok {
				name = m.Name
			}
			out.Reached = append(out.Reached, ReachedView{ID: r.MilestoneID, Name: name, ReachedAt: r.ReachedAt, ServerDay: int(r.ServerDay), Reason: Reason(r.Reason)})
		}
		return nil
	})
	return out, err
}

// OverrideInput replaces a milestone's override (nil = catalogue value). It takes effect on the next worker tick.
type OverrideInput struct {
	ThresholdPointsPerActive *int64 `json:"thresholdPointsPerActive"`
	TargetDay                *int   `json:"targetDay"`
	ForceReach               bool   `json:"forceReach"`
	Paused                   bool   `json:"paused"`
	Reason                   string `json:"reason"`
}

// SetOverride shifts (threshold / target day), forces or pauses an unreached milestone. Audited in the same tx
// (W9: admin_audit_logs 'world.milestone_override'). The world_state lock serializes it with the worker tick.
func (s *Service) SetOverride(ctx context.Context, admin uuid.UUID, milestoneID string, in OverrideInput) (OverrideView, error) {
	if _, ok := s.Cat.Milestone(milestoneID); !ok {
		return OverrideView{}, ErrUnknownMilestone
	}
	in.Reason = strings.TrimSpace(in.Reason)
	switch {
	case in.ThresholdPointsPerActive != nil && (*in.ThresholdPointsPerActive < 0 || *in.ThresholdPointsPerActive > maxThresholdPts):
		return OverrideView{}, fmt.Errorf("%w: thresholdPointsPerActive out of range 0..%d", ErrInvalidOverride, maxThresholdPts)
	case in.TargetDay != nil && (*in.TargetDay < 0 || *in.TargetDay > maxTargetDay):
		return OverrideView{}, fmt.Errorf("%w: targetDay out of range 0..%d", ErrInvalidOverride, maxTargetDay)
	case in.Reason == "" || len(in.Reason) > 500:
		return OverrideView{}, fmt.Errorf("%w: a reason (≤ 500 chars) is required", ErrInvalidOverride)
	}
	var out OverrideView
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		if _, err := q.GetWorldStateForUpdate(ctx); err != nil {
			return fmt.Errorf("lock world state: %w", err)
		}
		reached, err := q.ListMilestonesReached(ctx)
		if err != nil {
			return err
		}
		for _, r := range reached {
			if r.MilestoneID == milestoneID {
				return ErrMilestoneReached
			}
		}
		before := []byte("{}")
		if old, err := q.GetMilestoneOverride(ctx, milestoneID); err == nil {
			before, _ = json.Marshal(overrideView(old))
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		p := db.UpsertMilestoneOverrideParams{MilestoneID: milestoneID, ForceReach: in.ForceReach, Paused: in.Paused, UpdatedBy: admin, UpdatedAt: s.Now().UTC()}
		if in.ThresholdPointsPerActive != nil {
			p.ThresholdPointsPerActive = pgtype.Int8{Int64: *in.ThresholdPointsPerActive, Valid: true}
		}
		if in.TargetDay != nil {
			p.TargetDay = pgtype.Int4{Int32: int32(*in.TargetDay), Valid: true}
		}
		row, err := q.UpsertMilestoneOverride(ctx, p)
		if err != nil {
			return err
		}
		out = overrideView(row)
		after, _ := json.Marshal(out)
		return q.InsertAdminAudit(ctx, db.InsertAdminAuditParams{AdminUserID: admin, Action: "world.milestone_override", TargetType: "world_milestone",
			TargetID: milestoneID, Before: before, After: after, Reason: pgtype.Text{String: in.Reason, Valid: true}})
	})
	return out, err
}

// SetPaused pauses (no milestone is reached; days still close and fold) or resumes the story clock. Audited when it
// changes anything; repeating the same call is a no-op.
func (s *Service) SetPaused(ctx context.Context, admin uuid.UUID, paused bool, reason string) (State, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 500 {
		return State{}, fmt.Errorf("%w: a reason (≤ 500 chars) is required", ErrInvalidOverride)
	}
	var out State
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		before, err := q.GetWorldStateForUpdate(ctx)
		if err != nil {
			return fmt.Errorf("lock world state: %w", err)
		}
		now := s.Now().UTC()
		if (before.PausedAt != nil) == paused {
			out = stateOf(before, now)
			return nil
		}
		var at *time.Time
		action := "world.resume"
		if paused {
			at, action = &now, "world.pause"
		}
		ws, err := q.SetWorldPaused(ctx, at)
		if err != nil {
			return err
		}
		out = stateOf(ws, now)
		b, _ := json.Marshal(stateOf(before, now))
		a, _ := json.Marshal(out)
		return q.InsertAdminAudit(ctx, db.InsertAdminAuditParams{AdminUserID: admin, Action: action, TargetType: "world", TargetID: "1",
			Before: b, After: a, Reason: pgtype.Text{String: reason, Valid: true}})
	})
	return out, err
}
