package hall

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/contribution"
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

// Publisher fans a WS event out to every API instance (realtime.Publish). Best effort; nil disables.
type Publisher func(ctx context.Context, eventType, id string, data any) error

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
	Seat           int   `json:"seat"`
	AccruedSeconds int64 `json:"accruedSeconds"`
}

// State is GET /lecture/state and the answer to enter/leave.
type State struct {
	ServerNow            time.Time   `json:"serverNow"`
	CycleEpoch           time.Time   `json:"cycleEpoch"`
	CycleSeconds         int64       `json:"cycleSeconds"`
	Seats                int         `json:"seats"`
	Listeners            int64       `json:"listeners"` // everyone in the hall, seated or outside
	Seated               []Listener  `json:"seated"`
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

// Enter seats the player (keeps their seat if already inside) and returns the hall state.
func (s *Service) Enter(ctx context.Context, userID uuid.UUID) (State, error) {
	w, err := s.who(ctx, userID)
	if err != nil {
		return State{}, err
	}
	now := s.Now()
	s.sweep(ctx, now)
	l := Listener{CharacterID: w.charID.String(), Name: w.name, StageID: w.stageID}
	seat, isNew, err := s.Hall.Enter(ctx, l, now)
	if err != nil {
		return State{}, fmt.Errorf("hall enter: %w", err)
	}
	if isNew {
		l.Seat = seat
		s.publish(ctx, "lecture.enter", l.CharacterID, l)
	}
	return s.state(ctx, w.charID, now)
}

// Leave removes the player from the hall (unpaid accrual stays until it expires or is paid on return).
func (s *Service) Leave(ctx context.Context, userID uuid.UUID) error {
	w, err := s.who(ctx, userID)
	if err != nil {
		return err
	}
	seat, ok, err := s.Hall.Leave(ctx, w.charID.String())
	if err != nil {
		return fmt.Errorf("hall leave: %w", err)
	}
	if ok {
		s.publish(ctx, "lecture.leave", w.charID.String(), Listener{CharacterID: w.charID.String(), Seat: seat})
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
	seat, _, err := s.Hall.SeatOf(ctx, w.charID.String())
	if err != nil {
		return Me{}, err
	}
	return Me{InHall: true, Seat: seat, AccruedSeconds: secs}, nil
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
	seated, total, err := s.Hall.Snapshot(ctx)
	if err != nil {
		return State{}, fmt.Errorf("hall snapshot: %w", err)
	}
	cur, open := s.Cfg.SlotAt(now)
	st := State{
		ServerNow: now.UTC(), CycleEpoch: s.Cfg.Epoch.UTC(), Seats: s.Cfg.Seats, Listeners: total, Seated: seated,
		ChatChannel: s.Cfg.ChatChannel, HeartbeatSeconds: int64(s.Cfg.Heartbeat / time.Second),
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
	if seat, in, err := s.Hall.SeatOf(ctx, charID.String()); err != nil {
		return State{}, err
	} else if in {
		st.Me = Me{InHall: true, Seat: seat}
		if secs, err := s.Hall.Beat(ctx, charID.String(), now); err == nil {
			st.Me.AccruedSeconds = secs
		}
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
		s.publish(ctx, "lecture.leave", l.CharacterID, l)
	}
}

func (s *Service) publish(ctx context.Context, typ, id string, data any) {
	if s.Publish == nil {
		return
	}
	if err := s.Publish(ctx, typ, id, data); err != nil && s.Log != nil {
		s.Log.Warn("lecture: publish", "type", typ, "err", err)
	}
}
