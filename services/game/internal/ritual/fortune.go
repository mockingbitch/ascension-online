package ritual

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/db"
	"tiendao/game/internal/fortune"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/platform/postgres"
)

// Drawn is today's gieo quẻ draw.
type Drawn struct {
	Outcome fortune.Outcome `json:"outcome"`
	RollBP  int64           `json:"rollBp"`
	DrawnAt time.Time       `json:"drawnAt"`
}

// BuffView is the active rate buff.
type BuffView struct {
	BonusBP  int64     `json:"bonusBp"`
	StartsAt time.Time `json:"startsAt"`
	EndsAt   time.Time `json:"endsAt"`
}

// FortuneView is GET /fortune and POST /fortune/draw: the disclosed odds table, today's draw (nil = not drawn yet) and the
// active buff (nil = none).
type FortuneView struct {
	Name     string                `json:"name"`
	Outcomes []fortune.Outcome     `json:"outcomes"`
	Today    *Drawn                `json:"today"`
	Buff     *BuffView             `json:"buff"`
	GameDay  string                `json:"gameDay"`
	ResetsAt time.Time             `json:"resetsAt"`
	Replayed bool                  `json:"replayed,omitempty"`
	Snapshot *cultivation.Snapshot `json:"cultivation,omitempty"` // draw response only
}

// Fortune owns the gieo quẻ draw transaction (catalogue: internal/fortune).
type Fortune struct {
	Pool        *pgxpool.Pool
	Cat         *fortune.Catalogue
	Cultivation *cultivation.Service
	RNG         fortune.RNG
	Now         func() time.Time
}

// View returns the odds table and today's state (read-only).
func (s *Fortune) View(ctx context.Context, userID uuid.UUID) (FortuneView, error) {
	q := db.New(s.Pool)
	ch, err := q.GetCharacterByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return FortuneView{}, ErrNoCharacter
	} else if err != nil {
		return FortuneView{}, err
	}
	return s.view(ctx, q, ch.ID, s.Now().UTC())
}

func (s *Fortune) view(ctx context.Context, q *db.Queries, charID uuid.UUID, now time.Time) (FortuneView, error) {
	v := FortuneView{Name: s.Cat.Name, Outcomes: s.Cat.All(), GameDay: gameday.Of(now).Format(time.DateOnly), ResetsAt: gameday.NextReset(now).UTC()}
	if f, err := q.GetDailyFortune(ctx, db.GetDailyFortuneParams{CharacterID: charID, GameDay: gameday.PG(now)}); err == nil {
		d := Drawn{RollBP: int64(f.RollBp), DrawnAt: f.CreatedAt}
		if o, ok := s.Cat.Get(f.OutcomeID); ok {
			d.Outcome = *o
		} else { // outcome removed from the catalogue after the draw: keep the stored bonus
			d.Outcome = fortune.Outcome{ID: f.OutcomeID, Name: f.OutcomeID, RateBonusBP: int64(f.BonusBp)}
		}
		v.Today = &d
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return v, err
	}
	b, err := s.Cultivation.Buff(ctx, q, charID)
	if err != nil {
		return v, err
	}
	if b.BonusBP > 0 && now.Before(b.End) {
		v.Buff = &BuffView{BonusBP: b.BonusBP, StartsAt: b.Start, EndsAt: b.End}
	}
	return v, nil
}

// Draw rolls today's fortune once per game day (PK character, game_day). Order: lock progression → guard (today's
// row exists → Replayed) → settle idle time first at the old rate (the buff is never retroactive) → server roll
// (injected RNG) → daily_fortunes row → character_buffs [now, next reset) with the outcome's bonus.
func (s *Fortune) Draw(ctx context.Context, userID uuid.UUID) (FortuneView, error) {
	var out FortuneView
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		ch, err := q.GetCharacterByUser(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoCharacter
		} else if err != nil {
			return err
		}
		p, err := q.GetProgressionForUpdate(ctx, ch.ID)
		if err != nil {
			return fmt.Errorf("lock progression: %w", err)
		}
		now := s.Now().UTC()
		gd := gameday.PG(now)
		if _, err := q.GetDailyFortune(ctx, db.GetDailyFortuneParams{CharacterID: ch.ID, GameDay: gd}); err == nil {
			out, err = s.view(ctx, q, ch.ID, now)
			out.Replayed = true
			return err
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		dayKey := gameday.Of(now).Format("20060102")
		if p, _, err = s.Cultivation.SettleLocked(ctx, q, userID, p, "fortune:"+dayKey); err != nil {
			return err
		}
		roll, o := s.Cat.Draw(s.RNG)
		if err := q.InsertDailyFortune(ctx, db.InsertDailyFortuneParams{CharacterID: ch.ID, GameDay: gd, OutcomeID: o.ID,
			RollBp: int32(roll), BonusBp: int32(o.RateBonusBP), CreatedAt: now}); err != nil {
			return fmt.Errorf("insert fortune: %w", err)
		}
		// Never retroactive: the buff starts no earlier than the settled timestamp (a settlement skipped as too
		// recent leaves last_cultivated_at in the past; a future one — clock anomaly — moves the start forward).
		start := now
		if p.LastCultivatedAt.After(start) {
			start = p.LastCultivatedAt
		}
		if end := gameday.NextReset(now).UTC(); o.RateBonusBP > 0 && start.Before(end) {
			if err := q.UpsertCharacterBuff(ctx, db.UpsertCharacterBuffParams{CharacterID: ch.ID, Slot: cultivation.BuffSlotFortune,
				BonusBp: int32(o.RateBonusBP), StartsAt: start, EndsAt: end, SourceRef: "fortune:" + dayKey}); err != nil {
				return fmt.Errorf("upsert buff: %w", err)
			}
		}
		if out, err = s.view(ctx, q, ch.ID, now); err != nil {
			return err
		}
		snap, err := s.Cultivation.SnapshotLocked(ctx, q, userID, p, now)
		out.Snapshot = &snap
		return err
	})
	return out, err
}
