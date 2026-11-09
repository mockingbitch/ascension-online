package leaderboard

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"tiendao/game/internal/appearance"
	"tiendao/game/internal/db"
	"tiendao/game/internal/realm"
)

// ErrUnavailable means the Redis cache is down; Postgres stays correct and the cache is rebuilt later.
var ErrUnavailable = errors.New("leaderboard: temporarily unavailable")

// MaxPageSize bounds a single page.
const MaxPageSize = 100

// Entry is one leaderboard row.
type Entry struct {
	Rank           int64     `json:"rank"` // 1-based
	CharacterID    uuid.UUID `json:"characterId"`
	Name           string    `json:"name"`
	StageID        string    `json:"stageId"`
	Score          float64   `json:"score"`
	StageReachedAt time.Time `json:"stageReachedAt"`
	// Equipped cosmetics (cosmetics.yaml ids) so others see the frame/title next to the portrait. Display only:
	// never part of the score.
	AvatarFrameID string `json:"avatarFrameId,omitempty"`
	TitleID       string `json:"titleId,omitempty"`
	// Chosen look (appearances.yaml) resolved for the row's path. Display only.
	Look *appearance.Look `json:"look,omitempty"`
}

// Service reads/writes the global realm leaderboard.
type Service struct {
	Pool   *pgxpool.Pool
	Redis  *redis.Client
	Realms *realm.Catalogue
	Key    string // defaults to KeyGlobalRealm
	// Look filters the frame/title shown on a row by the player's điểm duyên (cosmetic.Catalogue.BoardFilter): Tiên
	// Duyên perks above the current level are hidden. nil → shown as equipped. Display only, never the score.
	Look func(frame, title string, vipPoints int64) (string, string)
	// Looks resolves each row's body / accessory / robe / pose for its path. nil → rows carry no look.
	Looks *appearance.Catalogue
}

func (s *Service) key() string {
	if s.Key != "" {
		return s.Key
	}
	return KeyGlobalRealm
}

// Refresh recomputes scores for the given characters FROM POSTGRES (not from event payloads, so a
// replayed old event can never overwrite a newer score) and writes them to Redis.
func (s *Service) Refresh(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	rows, err := db.New(s.Pool).GetProgressionsByIDs(ctx, ids)
	if err != nil {
		return err
	}
	zs := make([]redis.Z, 0, len(rows))
	for _, r := range rows {
		st, err := s.Realms.Stage(r.StageID)
		if err != nil {
			return err
		}
		zs = append(zs, redis.Z{Score: Score(st, r.CultivationMilli), Member: r.CharacterID.String()})
	}
	if len(zs) == 0 {
		return nil
	}
	return s.Redis.ZAdd(ctx, s.key(), zs...).Err()
}

// Rebuild reconstructs the whole leaderboard from Postgres into a temp key and atomically swaps it in.
// Run periodically (reconcile) and after catalogue retunes.
func (s *Service) Rebuild(ctx context.Context) (int, error) {
	tmp := s.key() + ":rebuild:" + strconv.FormatInt(time.Now().UnixNano(), 36)
	q := db.New(s.Pool)
	after, total := uuid.Nil, 0
	for {
		rows, err := q.ListProgressionsPage(ctx, db.ListProgressionsPageParams{After: after, Lim: 1000})
		if err != nil {
			return total, err
		}
		if len(rows) == 0 {
			break
		}
		zs := make([]redis.Z, 0, len(rows))
		for _, r := range rows {
			st, err := s.Realms.Stage(r.StageID)
			if err != nil {
				return total, err
			}
			zs = append(zs, redis.Z{Score: Score(st, r.CultivationMilli), Member: r.CharacterID.String()})
		}
		pipe := s.Redis.TxPipeline()
		pipe.ZAdd(ctx, tmp, zs...)
		pipe.Expire(ctx, tmp, 10*time.Minute) // a crashed rebuild must not leak keys
		if _, err := pipe.Exec(ctx); err != nil {
			return total, err
		}
		total += len(rows)
		after = rows[len(rows)-1].CharacterID
	}
	if total == 0 {
		return 0, s.Redis.Del(ctx, s.key()).Err()
	}
	if err := s.Redis.Rename(ctx, tmp, s.key()).Err(); err != nil {
		return total, err
	}
	return total, s.Redis.Persist(ctx, s.key()).Err()
}

// Page returns ranks [offset+1, offset+limit]. Ties inside the page are ordered by who reached the stage
// first, then character ID (Redis alone would order ties by member string).
func (s *Service) Page(ctx context.Context, offset, limit int64) ([]Entry, int64, error) {
	limit = max(1, min(limit, MaxPageSize))
	offset = max(0, offset)
	zs, err := s.Redis.ZRevRangeWithScores(ctx, s.key(), offset, offset+limit-1).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	total, err := s.Redis.ZCard(ctx, s.key()).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	entries, err := s.hydrate(ctx, zs)
	if err != nil {
		return nil, 0, err
	}
	slices.SortStableFunc(entries, func(a, b Entry) int {
		switch {
		case a.Score != b.Score:
			if a.Score > b.Score {
				return -1
			}
			return 1
		case !a.StageReachedAt.Equal(b.StageReachedAt):
			return a.StageReachedAt.Compare(b.StageReachedAt)
		default:
			return slices.Compare(a.CharacterID[:], b.CharacterID[:])
		}
	})
	for i := range entries {
		entries[i].Rank = offset + int64(i) + 1
	}
	return entries, total, nil
}

// RankOf returns a character's rank (1-based) or 0 if unranked.
func (s *Service) RankOf(ctx context.Context, characterID uuid.UUID) (int64, float64, error) {
	r, err := s.Redis.ZRevRank(ctx, s.key(), characterID.String()).Result()
	if errors.Is(err, redis.Nil) {
		return 0, 0, nil
	} else if err != nil {
		return 0, 0, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	score, err := s.Redis.ZScore(ctx, s.key(), characterID.String()).Result()
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return r + 1, score, nil
}

func (s *Service) hydrate(ctx context.Context, zs []redis.Z) ([]Entry, error) {
	ids := make([]uuid.UUID, 0, len(zs))
	scores := make(map[uuid.UUID]float64, len(zs))
	for _, z := range zs {
		id, err := uuid.Parse(fmt.Sprint(z.Member))
		if err != nil {
			continue
		}
		ids = append(ids, id)
		scores[id] = z.Score
	}
	rows, err := db.New(s.Pool).GetLeaderboardRows(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(rows))
	for _, r := range rows {
		frame, title := r.AvatarFrameID.String, r.TitleID.String
		if s.Look != nil {
			frame, title = s.Look(frame, title, r.VipPoints)
		}
		e := Entry{CharacterID: r.ID, Name: r.Name, StageID: r.StageID, Score: scores[r.ID], StageReachedAt: r.StageReachedAt,
			AvatarFrameID: frame, TitleID: title}
		if s.Looks != nil {
			if st, err := s.Realms.Stage(r.StageID); err == nil {
				l := s.Looks.Resolve(appearance.Look{BodyID: r.BodyID.String, HairID: r.HairID.String,
					RobePaletteID: r.RobePaletteID.String, PoseID: r.PoseID.String}, st.Path)
				e.Look = &l
			}
		}
		out = append(out, e)
	}
	return out, nil
}
