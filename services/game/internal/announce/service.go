package announce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"tiendao/game/internal/db"
	"tiendao/game/internal/realm"
	"tiendao/game/internal/realtime"
)

// Redis keys (all rebuildable / safe to lose: losing one costs at most one extra or one missing line of news).
const (
	keyWindow   = "announce:win:"           // + window start (unix s): separate announcements counted in that window
	keyDigest   = "announce:digest:"        // + window start: LIST of folded entries (JSON)
	keyPending  = "announce:digest:pending" // SET of window starts with a digest to flush
	keyRankCool = "announce:rank:"          // + characterId: rank news cooldown
	keySeen     = "announce:seen:"          // + outbox event id: already announced or folded (a replayed drain is a no-op)
)

// Item is one announcement as served (GET /announcements/recent) and published (WS data). Public fields only.
type Item struct {
	ID            int64        `json:"id"`
	Kind          string       `json:"kind"`
	Tier          int          `json:"tier"`
	CharacterID   *uuid.UUID   `json:"characterId,omitempty"`
	CharacterName string       `json:"characterName"`
	RealmID       string       `json:"realmId,omitempty"`
	StageID       string       `json:"stageId,omitempty"`
	Path          realm.PathID `json:"path,omitempty"`
	Tribulation   bool         `json:"tribulation"`
	Rank          int          `json:"rank,omitempty"`
	Count         int          `json:"count,omitempty"`
	Text          string       `json:"text"`
	CreatedAt     time.Time    `json:"createdAt"`
}

// TypeOf is the WS envelope type of an item kind.
func TypeOf(kind string) string {
	switch kind {
	case KindFirst:
		return TypeFirst
	case KindRank:
		return TypeRank
	case KindDigest:
		return TypeDigest
	default:
		return TypeBreakthrough
	}
}

// Service renders, paces, stores and publishes announcements (worker) and serves the recent history (API).
type Service struct {
	Pool   *pgxpool.Pool
	Redis  *redis.Client // nil (API) → no publishing
	Tpl    realtime.Templates
	Realms *realm.Catalogue
	Cfg    *Settings
	Now    func() time.Time
	Log    *slog.Logger
	// KeyPrefix isolates the Redis keys (tests); "" in production.
	KeyPrefix string
	// PublishFn replaces realtime.Publish (tests: pub/sub is server-wide, so tests must not broadcast). nil = Publish.
	PublishFn func(ctx context.Context, e realtime.Envelope) error
}

func (s *Service) k(key string) string { return s.KeyPrefix + key }

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func (s *Service) fxTier(realmID string) int {
	for _, r := range s.Realms.Realms() {
		if r.ID == realmID {
			return r.Visual.FxTier
		}
	}
	return 0
}

func (s *Service) realmName(realmID string) string {
	for _, r := range s.Realms.Realms() {
		if r.ID == realmID {
			return r.Name
		}
	}
	return realmID
}

// store writes the history row in the caller's transaction (idempotent per outbox event) and returns the item.
func (s *Service) store(ctx context.Context, q *db.Queries, src int64, it Item) (Item, bool, error) {
	var cid pgtype.UUID
	if it.CharacterID != nil {
		cid = pgtype.UUID{Bytes: *it.CharacterID, Valid: true}
	}
	p := db.InsertWorldAnnouncementParams{Kind: it.Kind, Tier: int16(it.Tier), CharacterID: cid, Name: it.CharacterName,
		RealmID: text(it.RealmID), StageID: text(it.StageID), Path: text(string(it.Path)), Text: it.Text, Now: it.CreatedAt}
	if src > 0 {
		p.SourceEventID = pgtype.Int8{Int64: src, Valid: true}
	}
	if it.Rank > 0 {
		p.Rank = pgtype.Int4{Int32: int32(it.Rank), Valid: true}
	}
	if it.Count > 0 {
		p.Count = pgtype.Int4{Int32: int32(it.Count), Valid: true}
	}
	row, err := q.InsertWorldAnnouncement(ctx, p)
	if err != nil {
		return Item{}, false, fmt.Errorf("store announcement: %w", err)
	}
	it.ID = row.ID
	return it, row.Inserted, nil
}

func (s *Service) publish(ctx context.Context, it Item) {
	env := realtime.Envelope{Type: TypeOf(it.Kind), ID: strconv.FormatInt(it.ID, 10), TS: it.CreatedAt.UTC(), Data: it}
	var err error
	switch {
	case s.PublishFn != nil:
		err = s.PublishFn(ctx, env)
	case s.Redis != nil:
		err = realtime.Publish(ctx, s.Redis, env)
	default:
		return
	}
	if err != nil && s.Log != nil {
		s.Log.Warn("publish announcement", "kind", it.Kind, "err", err)
	}
}

func (s *Service) window(now time.Time) int64 {
	w := s.Cfg.WindowSeconds
	return now.Unix() / w * w
}

// separate reports whether this announcement may go out on its own in the current window (≤ max per window);
// otherwise it is folded into the window's digest. Redis down → separate (news is best effort).
func (s *Service) separate(ctx context.Context, now time.Time) (bool, int64) {
	w := s.window(now)
	key := s.k(keyWindow) + strconv.FormatInt(w, 10)
	// INCR + EXPIRE in one MULTI: a key never survives without its TTL
	pipe := s.Redis.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, time.Duration(3*s.Cfg.WindowSeconds)*time.Second)
	if _, err := pipe.Exec(ctx); err != nil {
		return true, w
	}
	return incr.Val() <= s.Cfg.MaxSeparatePerWindow, w
}

type folded struct {
	CharacterID string `json:"characterId"`
	Name        string `json:"name"`
	RealmID     string `json:"realmId"`
}

// Breakthrough handles one announcement.major_breakthrough outbox event. The worker calls it AFTER the drain
// transaction committed, with a plain connection: the history insert is its own statement, so a failure here never
// rolls back other outbox work, and nothing is published for a batch that rolled back (review H1/M1).
func (s *Service) Breakthrough(ctx context.Context, q *db.Queries, eventID int64, payload []byte) error {
	var e Event
	if err := json.Unmarshal(payload, &e); err != nil {
		return fmt.Errorf("announce payload: %w", err)
	}
	now := s.Now().UTC()
	tier := s.fxTier(e.RealmID)
	// at-most-once per outbox event: a replayed drain neither re-announces nor re-folds it
	if s.Redis != nil {
		if fresh, err := s.Redis.SetNX(ctx, s.k(keySeen)+strconv.FormatInt(eventID, 10), 1, 24*time.Hour).Result(); err == nil && !fresh {
			return nil
		}
	}
	// First-to-realm is never folded (once per realm id); the others respect the window budget.
	if !e.First && s.Redis != nil {
		if ok, w := s.separate(ctx, now); !ok {
			b, _ := json.Marshal(folded{CharacterID: e.CharacterID, Name: e.Name, RealmID: e.RealmID})
			ws := strconv.FormatInt(w, 10)
			pipe := s.Redis.TxPipeline()
			pipe.RPush(ctx, s.k(keyDigest)+ws, b)
			pipe.Expire(ctx, s.k(keyDigest)+ws, time.Hour)
			pipe.SAdd(ctx, s.k(keyPending), ws)
			_, _ = pipe.Exec(ctx)
			return nil
		}
	}
	it := Item{Kind: e.Kind(), Tier: tier, CharacterName: e.Name, RealmID: e.RealmID, StageID: e.StageID, Path: e.Path,
		Tribulation: e.Tribulation, CreatedAt: now,
		Text: Render(s.Tpl, TemplateKey(e, tier), map[string]string{"name": e.Name, "realm": s.realmName(e.RealmID)})}
	if id, err := uuid.Parse(e.CharacterID); err == nil {
		it.CharacterID = &id
	}
	it, inserted, err := s.store(ctx, q, eventID, it)
	if err != nil {
		return err
	}
	if inserted {
		s.publish(ctx, it)
	}
	return nil
}

// RankMove is one character's Thiên Bảng move observed by the worker (0 = unranked).
type RankMove struct {
	CharacterID uuid.UUID
	Name        string
	Old, New    int64
}

// Rank announces entries into the top (or rank 1), at most once per player per cooldown (Redis SET NX).
func (s *Service) Rank(ctx context.Context, q *db.Queries, moves []RankMove) error {
	if s.Redis == nil {
		return nil
	}
	now := s.Now().UTC()
	for _, m := range moves {
		key := RankNews(m.Old, m.New, s.Cfg.RankTop)
		if key == "" || m.Name == "" {
			continue
		}
		// rank news shares the window budget with breakthroughs (review M2): over it, the news is simply skipped
		// (it would only be a rank line; Thiên Bảng shows the truth anyway)
		if ok, _ := s.separate(ctx, now); !ok {
			continue
		}
		ok, err := s.Redis.SetNX(ctx, s.k(keyRankCool)+m.CharacterID.String(), 1, time.Duration(s.Cfg.RankCooldownSeconds)*time.Second).Result()
		if err != nil || !ok {
			continue
		}
		id := m.CharacterID
		it := Item{Kind: KindRank, CharacterID: &id, CharacterName: m.Name, Rank: int(m.New), CreatedAt: now,
			Text: Render(s.Tpl, key, map[string]string{"name": m.Name, "rank": strconv.FormatInt(m.New, 10)})}
		it, _, err = s.store(ctx, q, 0, it)
		if err != nil {
			return err
		}
		s.publish(ctx, it)
	}
	return nil
}

// FlushDigests publishes one digest per finished window that folded announcements (called by the worker tick).
func (s *Service) FlushDigests(ctx context.Context) error {
	if s.Redis == nil {
		return nil
	}
	now := s.Now().UTC()
	pending, err := s.Redis.SMembers(ctx, s.k(keyPending)).Result()
	if err != nil {
		return err
	}
	current := s.window(now)
	for _, ws := range pending {
		w, err := strconv.ParseInt(ws, 10, 64)
		if err != nil || w >= current {
			continue // still filling
		}
		// claim the window (one worker): SREM returns 1 only for the first caller
		if n, err := s.Redis.SRem(ctx, s.k(keyPending), ws).Result(); err != nil || n == 0 {
			continue
		}
		list, err := s.Redis.LRange(ctx, s.k(keyDigest)+ws, 0, -1).Result()
		s.Redis.Del(ctx, s.k(keyDigest)+ws)
		if err != nil || len(list) == 0 {
			continue
		}
		var first folded
		if json.Unmarshal([]byte(list[0]), &first) != nil {
			continue
		}
		it := Item{Kind: KindDigest, CharacterName: first.Name, Count: len(list) - 1, CreatedAt: now, Tier: s.fxTier(first.RealmID)}
		if len(list) == 1 {
			// a lone folded item: say it plainly
			it.Kind, it.RealmID = KindRealm, first.RealmID
			it.Text = Render(s.Tpl, TemplateKey(Event{}, it.Tier), map[string]string{"name": first.Name, "realm": s.realmName(first.RealmID)})
		} else {
			it.Text = Render(s.Tpl, "digest", map[string]string{"name": first.Name, "count": strconv.Itoa(len(list) - 1)})
		}
		if id, err := uuid.Parse(first.CharacterID); err == nil {
			it.CharacterID = &id
		}
		it, _, err = s.store(ctx, db.New(s.Pool), 0, it)
		if err != nil {
			return err
		}
		s.publish(ctx, it)
	}
	return nil
}

// Prune drops history older than history_keep_days.
func (s *Service) Prune(ctx context.Context) (int64, error) {
	return db.New(s.Pool).PruneWorldAnnouncements(ctx, s.Now().Add(-time.Duration(s.Cfg.HistoryKeepDays)*24*time.Hour))
}

// Recent is GET /announcements/recent: newest first, ≤ recent_limit (and ≤ limit when given).
func (s *Service) Recent(ctx context.Context, limit int32) ([]Item, error) {
	if limit <= 0 || limit > s.Cfg.RecentLimit {
		limit = s.Cfg.RecentLimit
	}
	rows, err := db.New(s.Pool).ListWorldAnnouncements(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]Item, 0, len(rows))
	for _, r := range rows {
		it := Item{ID: r.ID, Kind: r.Kind, Tier: int(r.Tier), CharacterName: r.Name, RealmID: r.RealmID.String, StageID: r.StageID.String,
			Path: realm.PathID(r.Path.String), Tribulation: r.Kind == KindTribulation, Text: r.Text, CreatedAt: r.CreatedAt}
		if r.CharacterID.Valid {
			id := uuid.UUID(r.CharacterID.Bytes)
			it.CharacterID = &id
		}
		if r.Rank.Valid {
			it.Rank = int(r.Rank.Int32)
		}
		if r.Count.Valid {
			it.Count = int(r.Count.Int32)
		}
		out = append(out, it)
	}
	return out, nil
}

// ErrUnavailable: no history store wired.
var ErrUnavailable = errors.New("announce: unavailable")
