package character

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/db"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/leaderboard"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/world"
)

var (
	ErrAlreadyExists = errors.New("character: account already has a character")
	ErrNameTaken     = errors.New("character: name already taken")
	ErrNotFound      = cultivation.ErrNoCharacter
)

// Currencies every character starts with (balance 0).
var startingCurrencies = []string{"spirit_stone", "premium"}

// View is the character screen payload. Cultivation is a read-only projection to "now" (pending idle settlement,
// caps, bottleneck and auto-advance included) — nothing is written.
type View struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	RealmID   string    `json:"realmId"`
	cultivation.Snapshot
	FailStreak          int32      `json:"failStreak"`
	CooldownUntil       *time.Time `json:"cooldownUntil"`
	StageReachedAt      time.Time  `json:"stageReachedAt"`
	OfflineBonusSeconds int32      `json:"offlineBonusSeconds"` // pending Tụ Linh Phù bonus
	BonusUsedToday      bool       `json:"bonusUsedToday"`
}

// Service creates and reads characters.
type Service struct {
	Pool        *pgxpool.Pool
	Cultivation *cultivation.Service
}

// Create makes the account's (single) character at the first stage of the default path. If the world started
// more than late_join_grant.after_server_day days ago, the one-time late-join grant (50% of the server median) is
// stored as grant_reserve_milli — at most once per account (late_join_grants PK), released by the credit path.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, rawName string) (View, error) {
	name, err := NormalizeName(rawName)
	if err != nil {
		return View{}, err
	}
	cat := s.Cultivation.Realms
	err = postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		if _, err := q.GetCharacterByUser(ctx, userID); err == nil {
			return ErrAlreadyExists
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		ch, err := q.CreateCharacter(ctx, db.CreateCharacterParams{UserID: userID, Name: name})
		if err != nil {
			return mapUnique(err)
		}
		now := s.Cultivation.Now().UTC()
		grant, err := s.lateJoinGrant(ctx, q, userID, ch.ID, now)
		if err != nil {
			return err
		}
		first := cat.First(cat.DefaultPath())
		if err := q.CreateProgression(ctx, db.CreateProgressionParams{
			CharacterID: ch.ID, RealmID: first.RealmID, StageID: first.ID, Now: now, GrantReserveMilli: grant,
		}); err != nil {
			return err
		}
		for _, cur := range startingCurrencies {
			if err := q.EnsureCurrencyAccount(ctx, db.EnsureCurrencyAccountParams{CharacterID: ch.ID, Currency: cur}); err != nil {
				return err
			}
		}
		if err := q.EnsureVIPAccount(ctx, userID); err != nil {
			return err
		}
		return cultivation.EnqueueLeaderboard(ctx, q, ch.ID, leaderboard.Score(first, 0))
	})
	if err != nil {
		return View{}, err
	}
	return s.Get(ctx, userID)
}

// lateJoinGrant inserts the account's one-time grant when eligible and returns its amount (0 otherwise).
func (s *Service) lateJoinGrant(ctx context.Context, q *db.Queries, userID, charID uuid.UUID, now time.Time) (int64, error) {
	ws, err := q.GetWorldState(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	day := world.ServerDay(ws.StartedAt, now)
	amount := s.Cultivation.Contribution.GrantMilli(ws.StartedAt != nil, day, ws.MedianPositionMilli)
	if amount <= 0 || !ws.MedianGameDay.Valid {
		return 0, nil
	}
	got, err := q.InsertLateJoinGrant(ctx, db.InsertLateJoinGrantParams{
		UserID: userID, CharacterID: pgtype.UUID{Bytes: charID, Valid: true}, ServerDay: int32(day),
		MedianPositionMilli: ws.MedianPositionMilli, MedianGameDay: ws.MedianGameDay, GrantMilli: amount,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil // this account already received its grant (ON CONFLICT DO NOTHING)
	}
	return got, err
}

// Get returns the character view with cultivation projected to now (nothing is written).
func (s *Service) Get(ctx context.Context, userID uuid.UUID) (View, error) {
	q := db.New(s.Pool)
	ch, err := q.GetCharacterByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return View{}, ErrNotFound
	} else if err != nil {
		return View{}, err
	}
	p, err := q.GetProgression(ctx, ch.ID)
	if err != nil {
		return View{}, err
	}
	proj, err := s.Cultivation.Project(ctx, q, userID, p)
	if err != nil {
		return View{}, err
	}
	reached := p.StageReachedAt
	if proj.Credit.After.Stage.ID != p.StageID {
		reached = s.Cultivation.Now().UTC()
	}
	return View{
		ID: ch.ID, Name: ch.Name, CreatedAt: ch.CreatedAt, RealmID: proj.Credit.After.Stage.RealmID, Snapshot: proj.Snapshot,
		FailStreak: p.BreakthroughFailStreak, CooldownUntil: p.BreakthroughCooldownUntil,
		StageReachedAt: reached, OfflineBonusSeconds: p.OfflineBonusSeconds,
		BonusUsedToday: gameday.Same(p.OfflineBonusUsedDay, s.Cultivation.Now()),
	}, nil
}

func mapUnique(err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		switch pe.ConstraintName {
		case "characters_name_lower_uq":
			return ErrNameTaken
		case "characters_user_id_key":
			return ErrAlreadyExists
		}
	}
	return err
}
