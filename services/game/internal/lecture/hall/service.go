package hall

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/appearance"
	"tiendao/game/internal/contribution"
	"tiendao/game/internal/cosmetic"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/dailytask"
	"tiendao/game/internal/db"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/lecture"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/vip"
)

// Domain errors (mapped to HTTP in httpapi).
var (
	ErrNoCharacter    = errors.New("hall: character not found")
	ErrInsightClosed  = errors.New("hall: no ngộ đạo moment is open")
	ErrNothingToPay   = errors.New("hall: no listening time accrued")
	ErrAlreadyClaimed = errors.New("hall: this ngộ đạo moment was already claimed")
)

// LeaveEvent is the data of lecture.leave: who left which cushion. MovedTo > 0 = they were moved to that điện
// (switch or merge) — the moved client re-reads its state and follows its new topic.
type LeaveEvent struct {
	CharacterID string `json:"characterId"`
	HallID      int    `json:"hallId"`
	Seat        int    `json:"seat"`
	MovedTo     int    `json:"movedTo,omitempty"`
}

// CountEvent is the data of lecture.count, pushed to each điện's sockets every hall.count_push_seconds.
type CountEvent struct {
	Listeners int64 `json:"listeners"` // everyone listening, all điện
	Halls     int   `json:"halls"`     // open điện
	HallID    int   `json:"hallId"`
	Count     int   `json:"count"` // listeners in this điện
}

// Publisher fans a WS event out to every API instance (realtime.PublishTopic). topic "" = every socket; hall events
// go to Topic(điện) only. Best effort (at-most-once); nil disables.
type Publisher func(ctx context.Context, topic, eventType, id string, data any) error

// Topic is the WS topic of one điện: its sockets get lecture.enter/leave/count and hall chat (decisions 2026-11-10).
func Topic(hallID int) string { return TopicPrefix + strconv.Itoa(hallID) }

// TopicPrefix is shared by every điện topic (the WS hub validates subscriptions against it).
const TopicPrefix = "hall:"

// WS event types of the hall (packages/contracts/ws-messages.md).
const (
	EventEnter = "lecture.enter"
	EventLeave = "lecture.leave"
	EventCount = "lecture.count"
)

// Service owns the hall (Redis) and the ngộ đạo payment transaction (Postgres).
type Service struct {
	Pool        *pgxpool.Pool
	Hall        *Hall
	Cfg         *lecture.Config
	Scripts     *lecture.Scripts // nil → no script in responses
	Cultivation *cultivation.Service
	VIP         *vip.Table // điểm duyên for ngộ đạo milestones (vip.yaml lecture_attendance)
	Now         func() time.Time
	Publish     Publisher
	Log         *slog.Logger
	// Looks resolves the listener's look for the seat (appearances.yaml). nil → seats carry no look.
	Looks *appearance.Catalogue
}

// InsightView tells the client when ngộ đạo opens (server clock) and whether this player already took it.
type InsightView struct {
	Current lecture.Slot `json:"current"`
	Open    bool         `json:"open"`
	Claimed bool         `json:"claimed"` // the current slot is already paid for this character
	Next    lecture.Slot `json:"next"`
	Every   int64        `json:"everySeconds"`
}

// TodayView is the lecture share of today's tu vi.
type TodayView struct {
	GameDay  string `json:"gameDay"`
	Milli    int64  `json:"milli"`
	CapMilli int64  `json:"capMilli"`
}

// Me is the viewer's own hall state.
type Me struct {
	InHall         bool  `json:"inHall"`
	HallID         int   `json:"hallId"` // 0 when not in the hall
	Seat           int   `json:"seat"`   // -1 when not in the hall
	AccruedSeconds int64 `json:"accruedSeconds"`
	// Display hints for the hall (whose names a phone shows first): the viewer's đạo lữ and faction ("" = none).
	PartnerID string `json:"partnerId,omitempty"`
	FactionID string `json:"factionId,omitempty"`
}

// State is GET /lecture/state and the answer to enter/switch.
type State struct {
	ServerNow    time.Time `json:"serverNow"`
	CycleEpoch   time.Time `json:"cycleEpoch"`
	CycleSeconds int64     `json:"cycleSeconds"`
	// HallID is the điện shown: the viewer's own when inside, else 0 (Seated is then empty).
	HallID               int         `json:"hallId"`
	Capacity             int         `json:"capacity"`        // cushions per điện (lectures.yaml hall.seats)
	Count                int         `json:"count"`           // listeners in this điện
	GlobalListeners      int64       `json:"globalListeners"` // listeners in every điện
	Halls                []HallCount `json:"halls"`           // open điện with their counts (picker), by number
	Seated               []Listener  `json:"seated"`          // this điện only
	ChatChannel          string      `json:"chatChannel"`
	HeartbeatSeconds     int64       `json:"heartbeatSeconds"`
	RateMilliPerHour     int64       `json:"rateMilliPerHour"`
	MaxSecondsPerInsight int64       `json:"maxSecondsPerInsight"`
	Insight              InsightView `json:"insight"`
	Today                TodayView   `json:"today"`
	Me                   Me          `json:"me"`
}

// InsightResult is POST /lecture/insight.
type InsightResult struct {
	Cultivation     cultivation.Snapshot `json:"cultivation"`
	PresenceSeconds int64                `json:"presenceSeconds"`
	RawMilli        int64                `json:"rawMilli"`
	EarnedMilli     int64                `json:"earnedMilli"` // after bucket/ceiling/bottleneck
	VIPPoints       int64                `json:"vipPoints"`
	StagesAdvanced  []string             `json:"stagesAdvanced"`
	LectureID       string               `json:"lectureId"`
	Takeaway        string               `json:"takeaway"`
	Today           TodayView            `json:"today"`
	Replayed        bool                 `json:"replayed"`
}

// NoteEntry is one lecture in the player's Đạo ngộ notebook.
type NoteEntry struct {
	LectureID string    `json:"lectureId"`
	Title     string    `json:"title"`
	Theme     string    `json:"theme"`
	Takeaway  string    `json:"takeaway"`
	FirstAt   time.Time `json:"firstAt"`
	Times     int64     `json:"times"`
}

type who struct {
	charID  uuid.UUID
	name    string
	stageID string
}

// affinity reads the assignment hints (faction, đạo lữ). Only Enter needs them.
func (s *Service) affinity(ctx context.Context, charID uuid.UUID) (Affinity, error) {
	r, err := db.New(s.Pool).GetLectureAffinity(ctx, charID)
	if err != nil {
		return Affinity{}, fmt.Errorf("hall affinity: %w", err)
	}
	return Affinity{PartnerID: r.PartnerID, FactionID: r.FactionID}, nil
}

func (s *Service) who(ctx context.Context, userID uuid.UUID) (who, error) {
	q := db.New(s.Pool)
	ch, err := q.GetCharacterByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return who{}, ErrNoCharacter
	} else if err != nil {
		return who{}, err
	}
	p, err := q.GetProgression(ctx, ch.ID)
	if err != nil {
		return who{}, fmt.Errorf("read progression: %w", err)
	}
	return who{charID: ch.ID, name: ch.Name, stageID: p.StageID}, nil
}

// lookOf reads the seat look (only on Enter: heartbeats and ngộ đạo never need it). nil when looks are off.
func (s *Service) lookOf(ctx context.Context, w who) (*appearance.Look, error) {
	if s.Looks == nil {
		return nil, nil
	}
	stored, _, err := cosmetic.LoadLook(ctx, db.New(s.Pool), w.charID)
	if err != nil {
		return nil, err
	}
	st, err := s.Cultivation.Realms.Stage(w.stageID)
	if err != nil {
		return nil, nil
	}
	l := s.Looks.Resolve(stored, st.Path)
	return &l, nil
}

// Enter seats the player in a điện (keeps their cushion if already inside) and returns the hall state. The điện is
// chosen by the server (Hall.Enter): đạo lữ → faction → fullest non-full → a new one.
func (s *Service) Enter(ctx context.Context, userID uuid.UUID) (State, error) {
	w, err := s.who(ctx, userID)
	if err != nil {
		return State{}, err
	}
	now := s.Now()
	s.sweep(ctx, now)
	look, err := s.lookOf(ctx, w)
	if err != nil {
		return State{}, fmt.Errorf("hall look: %w", err)
	}
	aff, err := s.affinity(ctx, w.charID)
	if err != nil {
		return State{}, err
	}
	l := Listener{CharacterID: w.charID.String(), Name: w.name, StageID: w.stageID, Look: look, FactionID: aff.FactionID}
	hid, seat, isNew, err := s.Hall.Enter(ctx, l, aff, now)
	if errors.Is(err, ErrHallsFull) {
		return State{}, err
	} else if err != nil {
		return State{}, fmt.Errorf("hall enter: %w", err)
	}
	if isNew {
		l.HallID, l.Seat = hid, seat
		s.publish(ctx, Topic(hid), EventEnter, l.CharacterID, l)
	}
	return s.state(ctx, w.charID, now)
}

// Switch moves the player to another open điện with a free cushion (the picker). Idempotent by its target: moving
// to the điện one is already in changes nothing and announces nothing. No reward or accrual changes — listening
// time is global and keeps running.
func (s *Service) Switch(ctx context.Context, userID uuid.UUID, target int) (State, error) {
	if target < 1 || target > s.Cfg.MaxHalls {
		return State{}, ErrHallUnknown
	}
	w, err := s.who(ctx, userID)
	if err != nil {
		return State{}, err
	}
	now := s.Now()
	s.sweep(ctx, now)
	id := w.charID.String()
	from, seat, moved, err := s.Hall.Switch(ctx, id, target)
	if err != nil {
		return State{}, err
	}
	if moved {
		s.publish(ctx, Topic(from.HallID), EventLeave, id, LeaveEvent{CharacterID: id, HallID: from.HallID, Seat: from.Seat, MovedTo: target})
		if l, ok, err := s.Hall.Listener(ctx, id); err == nil && ok {
			l.Seat = seat
			s.publish(ctx, Topic(target), EventEnter, id, l)
		}
	}
	return s.state(ctx, w.charID, now)
}

// Leave removes the player from the hall (unpaid accrual stays until it expires or is paid on return).
func (s *Service) Leave(ctx context.Context, userID uuid.UUID) error {
	w, err := s.who(ctx, userID)
	if err != nil {
		return err
	}
	gone, ok, err := s.Hall.Leave(ctx, w.charID.String())
	if err != nil {
		return fmt.Errorf("hall leave: %w", err)
	}
	if ok && gone.HallID > 0 {
		s.publish(ctx, Topic(gone.HallID), EventLeave, gone.CharacterID, LeaveEvent{CharacterID: gone.CharacterID, HallID: gone.HallID, Seat: gone.Seat})
	}
	return nil
}

// Beat records a heartbeat. ErrNotPresent tells the client to Enter again (e.g. after a long sleep or Redis loss).
func (s *Service) Beat(ctx context.Context, userID uuid.UUID) (Me, error) {
	w, err := s.who(ctx, userID)
	if err != nil {
		return Me{}, err
	}
	now := s.Now()
	s.sweep(ctx, now)
	secs, err := s.Hall.Beat(ctx, w.charID.String(), now)
	if err != nil {
		return Me{}, err
	}
	hid, seat, ok, err := s.Hall.Where(ctx, w.charID.String())
	if err != nil {
		return Me{}, err
	}
	if !ok { // present (beat) but no cushion: should not happen — Enter again re-seats
		return Me{}, ErrNotPresent
	}
	return Me{InHall: true, HallID: hid, Seat: seat, AccruedSeconds: secs}, nil
}

// State returns the hall without entering it.
func (s *Service) State(ctx context.Context, userID uuid.UUID) (State, error) {
	w, err := s.who(ctx, userID)
	if err != nil {
		return State{}, err
	}
	now := s.Now()
	s.sweep(ctx, now)
	return s.state(ctx, w.charID, now)
}

func (s *Service) state(ctx context.Context, charID uuid.UUID, now time.Time) (State, error) {
	total, halls, err := s.Hall.Counts(ctx)
	if err != nil {
		return State{}, fmt.Errorf("hall counts: %w", err)
	}
	cur, open := s.Cfg.SlotAt(now)
	st := State{
		ServerNow: now.UTC(), CycleEpoch: s.Cfg.Epoch.UTC(), Capacity: s.Cfg.Seats, GlobalListeners: total, Halls: halls,
		Seated: []Listener{}, ChatChannel: s.Cfg.ChatChannel, HeartbeatSeconds: int64(s.Cfg.Heartbeat / time.Second),
		RateMilliPerHour: s.Cfg.RateMilliPerHour, MaxSecondsPerInsight: s.Cfg.MaxSecondsPerInsight,
		Insight: InsightView{Current: cur, Open: open, Next: s.Cfg.NextSlot(now), Every: int64(s.Cfg.InsightEvery / time.Second)},
		Me:      Me{Seat: -1},
	}
	if s.Scripts != nil {
		st.CycleSeconds = int64(s.Scripts.Total / time.Second)
	}
	q := db.New(s.Pool)
	if _, err := q.GetLectureInsight(ctx, db.GetLectureInsightParams{CharacterID: charID, SlotNo: cur.No}); err == nil {
		st.Insight.Claimed = true
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return State{}, err
	}
	if st.Today, err = s.today(ctx, q, charID, now); err != nil {
		return State{}, err
	}
	if hid, seat, in, err := s.Hall.Where(ctx, charID.String()); err != nil {
		return State{}, err
	} else if in {
		st.Me = Me{InHall: true, HallID: hid, Seat: seat}
		if secs, err := s.Hall.Beat(ctx, charID.String(), now); err == nil {
			st.Me.AccruedSeconds = secs
		}
		st.HallID = hid
		if aff, err := s.affinity(ctx, charID); err == nil {
			st.Me.PartnerID, st.Me.FactionID = aff.PartnerID, aff.FactionID
		}
		if st.Seated, err = s.Hall.Snapshot(ctx, hid); err != nil {
			return State{}, fmt.Errorf("hall snapshot: %w", err)
		}
		st.Count = len(st.Seated)
	}
	return st, nil
}

func (s *Service) today(ctx context.Context, q *db.Queries, charID uuid.UUID, now time.Time) (TodayView, error) {
	sum, err := q.SumLectureDay(ctx, db.SumLectureDayParams{CharacterID: charID, GameDay: gameday.PG(now)})
	if err != nil {
		return TodayView{}, err
	}
	return TodayView{GameDay: gameday.Of(now).Format(time.DateOnly), Milli: sum, CapMilli: s.Cfg.DailyCapMilli}, nil
}

// Insight pays the listening accrued since the last ngộ đạo, once per slot. Order: Redis take → DB transaction
// (lock progression → slot guard → daily cap → CreditLocked → điểm duyên milestones → insight row); if the
// transaction fails the seconds go back to Redis. A second call in the same slot returns the stored result.
func (s *Service) Insight(ctx context.Context, userID uuid.UUID) (InsightResult, error) {
	w, err := s.who(ctx, userID)
	if err != nil {
		return InsightResult{}, err
	}
	now := s.Now()
	slot, open := s.Cfg.SlotAt(now)
	if !open {
		return InsightResult{}, ErrInsightClosed
	}
	q := db.New(s.Pool)
	if prev, err := q.GetLectureInsight(ctx, db.GetLectureInsightParams{CharacterID: w.charID, SlotNo: slot.No}); err == nil {
		return s.replay(ctx, userID, prev, now)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return InsightResult{}, err
	}
	charKey := w.charID.String()
	secs, err := s.Hall.Take(ctx, charKey, now)
	if err != nil {
		return InsightResult{}, err
	}
	if secs <= 0 {
		// Another tab may have claimed this slot a moment ago: answer with its result rather than a 409.
		if prev, err := q.GetLectureInsight(ctx, db.GetLectureInsightParams{CharacterID: w.charID, SlotNo: slot.No}); err == nil {
			return s.replay(ctx, userID, prev, now)
		}
		return InsightResult{}, ErrNothingToPay
	}
	var lectureID, takeaway string
	if sc, _ := s.Scripts.At(s.Cfg.Epoch, now); sc != nil {
		lectureID, takeaway = sc.ID, sc.Takeaway
	}
	var out InsightResult
	var replayed *db.LectureInsight
	err = postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		p, err := q.GetProgressionForUpdate(ctx, w.charID)
		if err != nil {
			return fmt.Errorf("lock progression: %w", err)
		}
		// Checked again under the lock: concurrent duplicates serialize here.
		if prev, err := q.GetLectureInsight(ctx, db.GetLectureInsightParams{CharacterID: w.charID, SlotNo: slot.No}); err == nil {
			replayed = &prev
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		gd := gameday.PG(now)
		before, err := q.SumLectureDay(ctx, db.SumLectureDayParams{CharacterID: w.charID, GameDay: gd})
		if err != nil {
			return err
		}
		raw := s.Cfg.Pay(secs, before)
		slotID := strconv.FormatInt(slot.No, 10)
		var credit cultivation.CreditOutcome
		if raw > 0 {
			p, credit, err = s.Cultivation.CreditLocked(ctx, q, p, contribution.SourceLecture, raw,
				cultivation.Ref{Type: "lecture_insight", ID: slotID, IdemKey: slotID}, now)
			if err != nil {
				return err
			}
		}
		// Daily task "Nghe giảng" (kind lecture_attend, decision Phase 0 #13): a ngộ đạo after real listening
		// (presence ≥ attend_min_seconds) counts. Recorded BEFORE the điểm duyên below: lock order task row → vip_accounts,
		// the same as dailytask.Claim.
		if secs >= s.Cfg.AttendMinSeconds {
			if err := s.Cultivation.Tasks.Record(ctx, q, w.charID, now, dailytask.LectureAttend, 0); err != nil {
				return err
			}
		}
		var vipPts int64
		day := gameday.Of(now).Format("20060102")
		for _, m := range s.Cfg.MilestonesCrossed(before, before+raw) {
			// Key per (day, milestone): the same key the lecture used before vip.yaml v3, so replays stay replays.
			key := fmt.Sprintf("lecture:%s:%d", day, m/1000)
			got, err := s.VIP.EarnLocked(ctx, q, userID, vip.ActLecture, vip.Ref{Type: "lecture_insight", ID: slotID, IdemKey: key}, now)
			if err != nil {
				return fmt.Errorf("earn điểm duyên: %w", err)
			}
			if !got.Replayed {
				vipPts += got.Points
			}
		}
		if err := q.InsertLectureInsight(ctx, db.InsertLectureInsightParams{
			CharacterID: w.charID, SlotNo: slot.No, GameDay: gd, LectureID: lectureID, PresenceSeconds: int32(secs),
			RawMilli: raw, CreditedMilli: credit.Applied.CreditedMilli, VipPoints: int32(vipPts), CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("insert insight: %w", err)
		}
		snap, err := s.Cultivation.SnapshotLocked(ctx, q, userID, p, now)
		if err != nil {
			return err
		}
		out = InsightResult{
			Cultivation: snap, PresenceSeconds: secs, RawMilli: raw, EarnedMilli: credit.Applied.CreditedMilli,
			VIPPoints: vipPts, StagesAdvanced: []string{}, LectureID: lectureID, Takeaway: takeaway,
			Today: TodayView{GameDay: gameday.Of(now).Format(time.DateOnly), Milli: before + raw, CapMilli: s.Cfg.DailyCapMilli},
		}
		for _, st := range credit.Passed {
			out.StagesAdvanced = append(out.StagesAdvanced, st.ID)
		}
		return nil
	})
	if err != nil && replayed == nil {
		// The COMMIT may have succeeded even though we saw an error (connection lost): never refund a paid slot.
		if _, gerr := db.New(s.Pool).GetLectureInsight(context.WithoutCancel(ctx), db.GetLectureInsightParams{CharacterID: w.charID, SlotNo: slot.No}); gerr == nil {
			secs = 0
		}
	}
	if err != nil || replayed != nil {
		if rerr := s.Hall.Restore(context.WithoutCancel(ctx), charKey, secs); rerr != nil && s.Log != nil {
			s.Log.Warn("lecture: restore accrual", "characterId", charKey, "err", rerr)
		}
	}
	if err != nil {
		return InsightResult{}, err
	}
	if replayed != nil {
		return s.replay(ctx, userID, *replayed, now)
	}
	return out, nil
}

func (s *Service) replay(ctx context.Context, userID uuid.UUID, row db.LectureInsight, now time.Time) (InsightResult, error) {
	var out InsightResult
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		p, err := q.GetProgression(ctx, row.CharacterID)
		if err != nil {
			return err
		}
		snap, err := s.Cultivation.SnapshotLocked(ctx, q, userID, p, now)
		if err != nil {
			return err
		}
		today, err := s.today(ctx, q, row.CharacterID, now)
		if err != nil {
			return err
		}
		out = InsightResult{Cultivation: snap, PresenceSeconds: int64(row.PresenceSeconds), RawMilli: row.RawMilli,
			EarnedMilli: row.CreditedMilli, VIPPoints: int64(row.VipPoints), StagesAdvanced: []string{},
			LectureID: row.LectureID, Today: today, Replayed: true}
		if sc, ok := s.Scripts.Get(row.LectureID); ok && sc != nil {
			out.Takeaway = sc.Takeaway
		}
		return nil
	})
	return out, err
}

// Notebook lists the lectures the player has had a paid ngộ đạo in (the "Đạo ngộ" notebook).
func (s *Service) Notebook(ctx context.Context, userID uuid.UUID) ([]NoteEntry, error) {
	w, err := s.who(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := db.New(s.Pool).ListLectureNotebook(ctx, w.charID)
	if err != nil {
		return nil, err
	}
	out := make([]NoteEntry, 0, len(rows))
	for _, r := range rows {
		e := NoteEntry{LectureID: r.LectureID, FirstAt: r.FirstAt, Times: r.Times}
		if sc, ok := s.Scripts.Get(r.LectureID); ok && sc != nil {
			e.Title, e.Theme, e.Takeaway = sc.Title, sc.Theme, sc.Takeaway
		}
		out = append(out, e)
	}
	return out, nil
}

// sweep drops stale listeners and tells clients (lazy, piggybacks on hall requests: no per-player timer).
func (s *Service) sweep(ctx context.Context, now time.Time) {
	gone, err := s.Hall.Sweep(ctx, now)
	if err != nil {
		if s.Log != nil {
			s.Log.Warn("lecture: sweep", "err", err)
		}
		return
	}
	for _, l := range gone {
		if l.HallID > 0 {
			s.publish(ctx, Topic(l.HallID), EventLeave, l.CharacterID, LeaveEvent{CharacterID: l.CharacterID, HallID: l.HallID, Seat: l.Seat})
		}
	}
}

// lectureNo is the running lecture number (changes when one lecture ends); without scripts, the ngộ đạo slot.
func (s *Service) lectureNo(now time.Time) int64 {
	if s.Scripts != nil && s.Scripts.Total > 0 {
		return s.Scripts.Index(s.Cfg.Epoch, now)
	}
	slot, _ := s.Cfg.SlotAt(now)
	return slot.No
}

// Compact merges near-empty điện once per lecture boundary (Hall.Merge is a no-op for a number already merged, so
// every API instance may call it) and tells both điện: lecture.leave with movedTo on the old topic, lecture.enter on
// the new one.
func (s *Service) Compact(ctx context.Context, now time.Time) ([]Move, error) {
	moves, err := s.Hall.Merge(ctx, s.lectureNo(now))
	if err != nil {
		return nil, err
	}
	for _, m := range moves {
		s.publish(ctx, Topic(m.From), EventLeave, m.CharacterID, LeaveEvent{CharacterID: m.CharacterID, HallID: m.From, Seat: m.FromSeat, MovedTo: m.To})
		if l, ok, err := s.Hall.Listener(ctx, m.CharacterID); err == nil && ok {
			s.publish(ctx, Topic(m.To), EventEnter, m.CharacterID, l)
		}
	}
	if len(moves) > 0 && s.Log != nil {
		s.Log.Info("lecture: merged điện", "moved", len(moves))
	}
	return moves, nil
}

// LocalFanout is this API instance's WS hub: the topics its sockets follow and a local (no Redis) push.
type LocalFanout interface {
	LocalTopics(prefix string) []string
	PushTopic(topic, eventType, id string, data any) error
}

// RunPresence is the hall's one loop per API instance (not per player): every hall.count_push_seconds it sweeps
// stale listeners, merges near-empty điện after a lecture boundary, and pushes lecture.count to the điện topics
// that this instance's sockets follow. Blocks until ctx ends.
func (s *Service) RunPresence(ctx context.Context, hub LocalFanout) {
	t := time.NewTicker(s.Cfg.CountPush)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.tick(ctx, hub)
		}
	}
}

func (s *Service) tick(ctx context.Context, hub LocalFanout) {
	now := s.Now()
	s.sweep(ctx, now)
	if _, err := s.Compact(ctx, now); err != nil && s.Log != nil {
		s.Log.Warn("lecture: merge", "err", err)
	}
	if hub == nil {
		return
	}
	topics := hub.LocalTopics(TopicPrefix)
	if len(topics) == 0 {
		return
	}
	total, halls, err := s.Hall.Counts(ctx)
	if err != nil {
		if s.Log != nil {
			s.Log.Warn("lecture: counts", "err", err)
		}
		return
	}
	byID := make(map[int]int, len(halls))
	for _, h := range halls {
		byID[h.ID] = h.Count
	}
	id := strconv.FormatInt(now.Unix(), 10)
	for _, tp := range topics {
		n, err := strconv.Atoi(strings.TrimPrefix(tp, TopicPrefix))
		if err != nil {
			continue
		}
		_ = hub.PushTopic(tp, EventCount, id, CountEvent{Listeners: total, Halls: len(halls), HallID: n, Count: byID[n]})
	}
}

// HallOf is the character's current điện (chat: hall messages are stored and delivered per điện).
func (s *Service) HallOf(ctx context.Context, charID uuid.UUID) (int, bool, error) {
	h, _, ok, err := s.Hall.Where(ctx, charID.String())
	return h, ok, err
}

func (s *Service) publish(ctx context.Context, topic, typ, id string, data any) {
	if s.Publish == nil {
		return
	}
	if err := s.Publish(ctx, topic, typ, id, data); err != nil && s.Log != nil {
		s.Log.Warn("lecture: publish", "type", typ, "err", err)
	}
}

// ValidTopic reports whether topic names a điện this config can open ("hall:1" … "hall:<max_halls>"): the WS hub
// accepts only these subscriptions.
func ValidTopic(cfg *lecture.Config, topic string) bool {
	rest, ok := strings.CutPrefix(topic, TopicPrefix)
	if !ok || rest == "" || len(rest) > 5 || rest[0] == '0' {
		return false
	}
	n, err := strconv.Atoi(rest)
	return err == nil && n >= 1 && n <= cfg.MaxHalls && strconv.Itoa(n) == rest
}
