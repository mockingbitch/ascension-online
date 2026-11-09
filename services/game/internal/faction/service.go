package faction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/appearance"
	"tiendao/game/internal/db"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/leaderboard"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/realm"
	"tiendao/game/internal/world"
)

// TopicChanged is the outbox topic of every membership change (aggregate = character id). The worker refreshes the
// per-faction realm ZSETs and member counts from Postgres.
const TopicChanged = "faction.changed"

// historyShown is how many recent changes GET /factions/me returns.
const historyShown = 10

// Service owns character_faction / faction_history (F5 flow: progression → character_faction → history → outbox).
type Service struct {
	Pool    *pgxpool.Pool
	Cat     *Catalogue
	Realms  *realm.Catalogue
	World   *world.Catalogue
	Ranking *Ranking // nil → GET /factions without counts/merit from Redis (Postgres counts only)
	Now     func() time.Time
	// Settle settles idle cultivation for a progression row the caller has locked (cultivation.SettleLocked, injected
	// by cmd wiring — this package can't import cultivation). Run before every faction change so offline accrual is
	// credited — and stamped — to the faction it was earned in (M1). nil → no settle (tests of pure flows).
	Settle func(ctx context.Context, q *db.Queries, userID uuid.UUID, p db.CharacterProgression, idemKey string) (db.CharacterProgression, error)

	fallbackMu      sync.Mutex
	fallbackAt      time.Time
	fallbackMembers map[string]int64
}

// fallbackTTL caches the Postgres member counts used when Redis is down (L7).
const fallbackTTL = 15 * time.Second

// ErrIdempotencyReused: the Idempotency-Key was already used for a different faction change (L1).
var ErrIdempotencyReused = errors.New("faction: idempotency key reused for another change")

// replay returns the membership for an already-applied key, or ErrIdempotencyReused when the key belonged to another
// operation or target.
func (s *Service) replay(ctx context.Context, q *db.Queries, ch db.Character, h db.FactionHistory, reason, target, stageID string) (ChangeResult, error) {
	if h.Reason != reason || h.ToFactionID != target {
		return ChangeResult{}, ErrIdempotencyReused
	}
	cf, err := loadCF(ctx, q, ch.ID, false)
	if err != nil {
		return ChangeResult{}, err
	}
	m, err := s.membership(ctx, q, ch.ID, cf, stageID, s.Now().UTC())
	return ChangeResult{Membership: m, Replayed: true}, err
}

// settleFirst credits pending idle cultivation under the progression lock before the faction changes (M1).
func (s *Service) settleFirst(ctx context.Context, q *db.Queries, userID uuid.UUID, p db.CharacterProgression, idemKey string) (db.CharacterProgression, error) {
	if s.Settle == nil {
		return p, nil
	}
	np, err := s.Settle(ctx, q, userID, p, "faction:"+idemKey)
	if err != nil {
		return p, fmt.Errorf("settle before faction change: %w", err)
	}
	return np, nil
}

// ErrNoCharacter: the account has no character (this package can't import cultivation: content → faction).
var ErrNoCharacter = errors.New("faction: character not found")

// ErrInvalidLock is an admin lock request out of range.
var ErrInvalidLock = errors.New("faction: invalid lock request")

// EffectNotice is a scheduled world effect that touches the character's faction.
type EffectNotice struct {
	Kind       world.EffectKind `json:"kind"`
	From       string           `json:"from"`
	Into       []string         `json:"into,omitempty"`
	To         string           `json:"to,omitempty"`
	DueAt      time.Time        `json:"dueAt"`
	DeadlineAt *time.Time       `json:"deadlineAt,omitempty"`
}

// HistoryEntry is one faction_history row.
type HistoryEntry struct {
	From      *string   `json:"from"`
	To        string    `json:"to"`
	FromPath  *string   `json:"fromPath,omitempty"`
	ToPath    *string   `json:"toPath,omitempty"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"createdAt"`
}

// Membership is GET /factions/me and the state returned by every mutation (the client replaces, never merges).
type Membership struct {
	FactionID             *string        `json:"factionId"` // null = no faction yet
	JoinedAt              *time.Time     `json:"joinedAt"`
	CanSwitchAt           *time.Time     `json:"canSwitchAt"` // null = a switch is allowed now (when a member)
	SwitchCooldownSeconds int64          `json:"switchCooldownSeconds"`
	Locked                bool           `json:"locked"`
	LockReason            string         `json:"lockReason,omitempty"`
	LockedUntil           *time.Time     `json:"lockedUntil,omitempty"`
	MustChoose            []string       `json:"mustChoose"` // branches of a pending split (empty = nothing to choose)
	ChooseDeadline        *time.Time     `json:"chooseDeadline"`
	Path                  realm.PathID   `json:"path"`
	StageID               string         `json:"stageId"`
	Upcoming              *EffectNotice  `json:"upcoming"` // a scheduled split/migrate of the current faction
	History               []HistoryEntry `json:"history"`
}

// ChangeResult is the response of join / switch / split-choice.
type ChangeResult struct {
	Membership Membership `json:"membership"`
	Replayed   bool       `json:"replayed"`
}

// ChangedEvent is the outbox payload of faction.changed.
type ChangedEvent struct {
	CharacterID uuid.UUID `json:"characterId"`
	From        string    `json:"from,omitempty"`
	To          string    `json:"to"`
	Reason      string    `json:"reason"`
}

func (s *Service) eraSeq(ws db.WorldState) int { return eraSeqOf(s.World, ws) }

// eraSeqOf is the current era's catalogue order; NULL = first era (§5.4).
func eraSeqOf(eras EraIndex, ws db.WorldState) int {
	if seq, ok := eras.EraSeq(ws.EraID.String); ws.EraID.Valid && ok {
		return seq
	}
	return 0
}

func memberOf(cf *db.CharacterFaction) *Member {
	if cf == nil {
		return nil
	}
	return &Member{FactionID: cf.FactionID, LastSwitchAt: cf.LastSwitchAt, LockReason: cf.LockReason.String, LockedUntil: cf.LockedUntil}
}

// loadCF reads (optionally locks) the membership row; nil when the character has no faction.
func loadCF(ctx context.Context, q *db.Queries, charID uuid.UUID, lock bool) (*db.CharacterFaction, error) {
	var cf db.CharacterFaction
	var err error
	if lock {
		cf, err = q.GetCharacterFactionForUpdate(ctx, charID)
	} else {
		cf, err = q.GetCharacterFaction(ctx, charID)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("load membership: %w", err)
	}
	return &cf, nil
}

func character(ctx context.Context, q *db.Queries, userID uuid.UUID) (db.Character, error) {
	ch, err := q.GetCharacterByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ch, ErrNoCharacter
	}
	return ch, err
}

// Join puts a character without a faction into an open faction (F2, F3: converts the path when needed).
func (s *Service) Join(ctx context.Context, userID uuid.UUID, idemKey, factionID string, path realm.PathID) (ChangeResult, error) {
	return s.change(ctx, userID, idemKey, KindJoin, factionID, path)
}

// Switch moves a member to another open faction after the cooldown (F4) unless locked (F5).
func (s *Service) Switch(ctx context.Context, userID uuid.UUID, idemKey, factionID string, path realm.PathID) (ChangeResult, error) {
	return s.change(ctx, userID, idemKey, KindSwitch, factionID, path)
}

func (s *Service) change(ctx context.Context, userID uuid.UUID, idemKey string, kind Kind, target string, wantPath realm.PathID) (ChangeResult, error) {
	var out ChangeResult
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		var err error
		out, err = s.changeTx(ctx, db.New(tx), userID, idemKey, kind, target, wantPath)
		return err
	})
	return out, err
}

// JoinTx is Join inside the caller's transaction (onboarding: POST /character {name, factionId, path} creates the
// character and joins in one tx, data-model §9.6). Same locks, idempotency (faction_history key) and outbox as Join.
func (s *Service) JoinTx(ctx context.Context, q *db.Queries, userID uuid.UUID, idemKey, factionID string, path realm.PathID) (ChangeResult, error) {
	return s.changeTx(ctx, q, userID, idemKey, KindJoin, factionID, path)
}

// JoinedWithKey reports whether the character already joined `factionID` with this Idempotency-Key (replay check of
// the onboarding create). A key used for another change → ErrIdempotencyReused.
func JoinedWithKey(ctx context.Context, q *db.Queries, charID uuid.UUID, idemKey, factionID string) (bool, error) {
	h, err := q.GetFactionHistoryByKey(ctx, db.GetFactionHistoryByKeyParams{CharacterID: charID, IdempotencyKey: idemKey})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if h.Reason != string(KindJoin) || h.ToFactionID != factionID {
		return false, ErrIdempotencyReused
	}
	return true, nil
}

func (s *Service) changeTx(ctx context.Context, q *db.Queries, userID uuid.UUID, idemKey string, kind Kind, target string, wantPath realm.PathID) (ChangeResult, error) {
	var out ChangeResult
	err := func() error {
		ch, err := character(ctx, q, userID)
		if err != nil {
			return err
		}
		// §2.3: progression first (serializes concurrent joins of the same character), then character_faction.
		p, err := q.GetProgressionForUpdate(ctx, ch.ID)
		if err != nil {
			return fmt.Errorf("lock progression: %w", err)
		}
		if h, err := q.GetFactionHistoryByKey(ctx, db.GetFactionHistoryByKeyParams{CharacterID: ch.ID, IdempotencyKey: idemKey}); err == nil {
			out, err = s.replay(ctx, q, ch, h, string(kind), target, p.StageID)
			return err
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if p, err = s.settleFirst(ctx, q, userID, p, idemKey); err != nil {
			return err
		}
		cf, err := loadCF(ctx, q, ch.ID, true)
		if err != nil {
			return err
		}
		now := s.Now().UTC()
		ws, err := q.GetWorldStateForShare(ctx) // the era can't move under us (L2)
		if err != nil {
			return fmt.Errorf("world state: %w", err)
		}
		st, err := s.Realms.Stage(p.StageID)
		if err != nil {
			return err
		}
		path, err := s.Cat.CanJoin(s.World, Request{Kind: kind, Member: memberOf(cf), Target: target, EraSeq: s.eraSeq(ws),
			CurPath: st.Path, WantPath: wantPath, Now: now})
		if err != nil {
			return err
		}
		h := db.InsertFactionHistoryParams{CharacterID: ch.ID, ToFactionID: target, Reason: string(kind), IdempotencyKey: idemKey, CreatedAt: now}
		stageID := p.StageID
		if path != st.Path {
			to, err := s.Realms.Convert(st, path)
			if err != nil { // the target path has no stage at this tier/stage (e.g. a reserved realm)
				return fmt.Errorf("%w: %v", ErrPathNotAllowed, err)
			}
			if err := q.ConvertProgressionPath(ctx, db.ConvertProgressionPathParams{RealmID: to.RealmID, StageID: to.ID, CharacterID: ch.ID}); err != nil {
				return fmt.Errorf("convert path: %w", err)
			}
			payload, _ := json.Marshal(map[string]string{"fromStageId": st.ID, "fromPath": string(st.Path), "toPath": string(path), "factionId": target})
			if err := q.InsertProgressionEvent(ctx, db.InsertProgressionEventParams{CharacterID: ch.ID, Kind: "path_converted",
				StageID: pgtype.Text{String: to.ID, Valid: true}, Payload: payload, OccurredAt: now}); err != nil {
				return err
			}
			if err := enqueueLeaderboard(ctx, q, ch.ID, leaderboard.Score(to, p.CultivationMilli)); err != nil {
				return err
			}
			h.FromPath, h.ToPath = text(string(st.Path)), text(string(path))
			stageID = to.ID
		}
		var from string
		switch kind {
		case KindJoin:
			var last *time.Time
			if !s.Cat.Settings.FirstJoinFree {
				last = &now
			}
			if err := q.InsertCharacterFaction(ctx, db.InsertCharacterFactionParams{CharacterID: ch.ID, FactionID: target, JoinedAt: now, LastSwitchAt: last}); err != nil {
				return fmt.Errorf("insert membership: %w", err)
			}
			// Today's points (if any) now count toward the new faction's merit.
			if err := q.StampTodayFaction(ctx, db.StampTodayFactionParams{FactionID: text(target), CharacterID: ch.ID, GameDay: date(gameday.Of(now))}); err != nil {
				return err
			}
		case KindSwitch:
			from = cf.FactionID
			h.FromFactionID = text(from)
			if err := q.MoveCharacterFaction(ctx, db.MoveCharacterFactionParams{FactionID: target, JoinedAt: now, LastSwitchAt: &now, CharacterID: ch.ID}); err != nil {
				return fmt.Errorf("move membership: %w", err)
			}
		}
		if err := q.InsertFactionHistory(ctx, h); err != nil {
			return fmt.Errorf("insert history: %w", err)
		}
		if err := enqueueChanged(ctx, q, ChangedEvent{CharacterID: ch.ID, From: from, To: target, Reason: string(kind)}); err != nil {
			return err
		}
		if cf, err = loadCF(ctx, q, ch.ID, false); err != nil {
			return err
		}
		out.Membership, err = s.membership(ctx, q, ch.ID, cf, stageID, now)
		return err
	}()
	return out, err
}

// ChooseSplit moves a member with a pending split to one of its branches before the deadline. It is a world-forced
// move: it ignores and does not start the switch cooldown. A locked member can't choose (ErrLocked) and is placed by
// the deadline default like everyone who didn't choose.
func (s *Service) ChooseSplit(ctx context.Context, userID uuid.UUID, idemKey, branch string) (ChangeResult, error) {
	var out ChangeResult
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		ch, err := character(ctx, q, userID)
		if err != nil {
			return err
		}
		p, err := q.GetProgressionForUpdate(ctx, ch.ID) // §2.3 order, same as join/switch
		if err != nil {
			return fmt.Errorf("lock progression: %w", err)
		}
		if h, err := q.GetFactionHistoryByKey(ctx, db.GetFactionHistoryByKeyParams{CharacterID: ch.ID, IdempotencyKey: idemKey}); err == nil {
			out, err = s.replay(ctx, q, ch, h, "split_choice", branch, p.StageID)
			return err
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if p, err = s.settleFirst(ctx, q, userID, p, idemKey); err != nil {
			return err
		}
		cf, err := loadCF(ctx, q, ch.ID, true)
		if err != nil {
			return err
		}
		now := s.Now().UTC()
		switch {
		case cf == nil:
			return ErrNotMember
		case !cf.PendingSplitEffectID.Valid:
			return ErrNoPendingSplit
		case memberOf(cf).Locked(now):
			return ErrLocked // M3: a locked member can't choose; the deadline default places them
		}
		eff, err := q.GetWorldEffect(ctx, cf.PendingSplitEffectID.Int64)
		if err != nil {
			return fmt.Errorf("load split effect: %w", err)
		}
		var e world.Effect
		if err := json.Unmarshal(eff.Snapshot, &e); err != nil {
			return fmt.Errorf("split snapshot: %w", err)
		}
		if eff.DeadlineAt == nil || !now.Before(*eff.DeadlineAt) || eff.FinalizedAt != nil {
			return ErrChoiceClosed
		}
		if !slices.Contains(e.Into, branch) {
			return ErrNotABranch
		}
		if _, ok := s.Cat.Get(branch); !ok {
			return ErrUnknownFaction
		}
		ws, err := q.GetWorldStateForShare(ctx) // the era can't move under us (L2)
		if err != nil {
			return fmt.Errorf("world state: %w", err)
		}
		if !slices.Contains(s.Cat.OpenBranches(s.World, e.Into, s.eraSeq(ws)), branch) {
			return ErrFactionClosed // a branch whose own era started (it closed and was moved on) takes nobody new
		}
		if err := q.MoveCharacterFaction(ctx, db.MoveCharacterFactionParams{FactionID: branch, JoinedAt: now, LastSwitchAt: cf.LastSwitchAt, CharacterID: ch.ID}); err != nil {
			return fmt.Errorf("move membership: %w", err)
		}
		if err := q.InsertFactionHistory(ctx, db.InsertFactionHistoryParams{CharacterID: ch.ID, FromFactionID: text(cf.FactionID), ToFactionID: branch,
			Reason: "split_choice", WorldEffectID: pgtype.Int8{Int64: eff.ID, Valid: true}, IdempotencyKey: idemKey, CreatedAt: now}); err != nil {
			return fmt.Errorf("insert history: %w", err)
		}
		if err := enqueueChanged(ctx, q, ChangedEvent{CharacterID: ch.ID, From: cf.FactionID, To: branch, Reason: "split_choice"}); err != nil {
			return err
		}
		if cf, err = loadCF(ctx, q, ch.ID, false); err != nil {
			return err
		}
		out.Membership, err = s.membership(ctx, q, ch.ID, cf, p.StageID, now)
		return err
	})
	return out, err
}

// Me returns the caller's membership (read-only).
func (s *Service) Me(ctx context.Context, userID uuid.UUID) (Membership, error) {
	q := db.New(s.Pool)
	ch, err := character(ctx, q, userID)
	if err != nil {
		return Membership{}, err
	}
	p, err := q.GetProgression(ctx, ch.ID)
	if err != nil {
		return Membership{}, err
	}
	cf, err := loadCF(ctx, q, ch.ID, false)
	if err != nil {
		return Membership{}, err
	}
	return s.membership(ctx, q, ch.ID, cf, p.StageID, s.Now().UTC())
}

func (s *Service) membership(ctx context.Context, q *db.Queries, charID uuid.UUID, cf *db.CharacterFaction, stageID string, now time.Time) (Membership, error) {
	m := Membership{SwitchCooldownSeconds: int64(s.Cat.Settings.SwitchCooldown / time.Second), MustChoose: []string{}, StageID: stageID,
		History: []HistoryEntry{}}
	if st, err := s.Realms.Stage(stageID); err == nil {
		m.Path = st.Path
	}
	if cf == nil {
		return m, nil
	}
	id, joined := cf.FactionID, cf.JoinedAt
	m.FactionID, m.JoinedAt = &id, &joined
	mem := memberOf(cf)
	if at := s.Cat.CanSwitchAt(mem); at != nil && now.Before(*at) {
		m.CanSwitchAt = at
	}
	m.Locked = mem.Locked(now)
	if m.Locked {
		m.LockReason, m.LockedUntil = cf.LockReason.String, cf.LockedUntil
	}
	effects, err := q.ListPendingWorldEffects(ctx, 50)
	if err != nil {
		return m, err
	}
	ws, err := q.GetWorldState(ctx)
	if err != nil {
		return m, fmt.Errorf("world state: %w", err)
	}
	cur := s.eraSeq(ws)
	for _, e := range effects {
		var eff world.Effect
		if json.Unmarshal(e.Snapshot, &eff) != nil {
			continue
		}
		if cf.PendingSplitEffectID.Valid && e.ID == cf.PendingSplitEffectID.Int64 && e.FinalizedAt == nil {
			m.MustChoose, m.ChooseDeadline = s.Cat.OpenBranches(s.World, eff.Into, cur), e.DeadlineAt
		}
		if (e.Kind == string(world.EffectSplitFaction) || e.Kind == string(world.EffectMigrateFaction)) && eff.From == cf.FactionID && m.Upcoming == nil {
			m.Upcoming = &EffectNotice{Kind: world.EffectKind(e.Kind), From: eff.From, Into: s.upcomingInto(eff, cur), To: eff.To, DueAt: e.DueAt, DeadlineAt: e.DeadlineAt}
		}
	}
	rows, err := q.ListFactionHistory(ctx, db.ListFactionHistoryParams{CharacterID: charID, Lim: historyShown})
	if err != nil {
		return m, err
	}
	for _, r := range rows {
		m.History = append(m.History, HistoryEntry{From: ptr(r.FromFactionID), To: r.ToFactionID, FromPath: ptr(r.FromPath), ToPath: ptr(r.ToPath),
			Reason: r.Reason, CreatedAt: r.CreatedAt})
	}
	return m, nil
}

// ───────────────────────────── Admin lock ─────────────────────────────

// Lock reasons (character_faction.lock_reason).
var lockReasons = []string{"fengshen", "admin", "event"}

// MaxLockHours bounds a timed lock (0 = permanent).
const MaxLockHours = 24 * 365

// LockView is the admin response.
type LockView struct {
	CharacterID uuid.UUID  `json:"characterId"`
	FactionID   string     `json:"factionId"`
	LockReason  string     `json:"lockReason,omitempty"`
	LockedAt    *time.Time `json:"lockedAt"`
	LockedUntil *time.Time `json:"lockedUntil"`
}

func lockView(cf db.CharacterFaction) LockView {
	return LockView{CharacterID: cf.CharacterID, FactionID: cf.FactionID, LockReason: cf.LockReason.String, LockedAt: cf.LockedAt, LockedUntil: cf.LockedUntil}
}

// SetLock locks (reason ∈ fengshen|admin|event; hours 0 = permanent) or unlocks (reason "") a member's faction
// switches. Audited in the same tx ('faction.lock' / 'faction.unlock'). Errors: ErrNotMember, ErrInvalidLock.
func (s *Service) SetLock(ctx context.Context, admin, characterID uuid.UUID, reason string, hours int, note string) (LockView, error) {
	note = strings.TrimSpace(note)
	switch {
	case reason != "" && !slices.Contains(lockReasons, reason):
		return LockView{}, fmt.Errorf("%w: reason must be one of %v", ErrInvalidLock, lockReasons)
	case hours < 0 || hours > MaxLockHours || (reason == "" && hours != 0):
		return LockView{}, fmt.Errorf("%w: hours out of range 0..%d", ErrInvalidLock, MaxLockHours)
	case note == "" || len(note) > 500:
		return LockView{}, fmt.Errorf("%w: a note (≤ 500 chars) is required", ErrInvalidLock)
	}
	var out LockView
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cf, err := loadCF(ctx, q, characterID, true)
		if err != nil {
			return err
		}
		if cf == nil {
			return ErrNotMember
		}
		before := lockView(*cf)
		now := s.Now().UTC()
		p := db.SetFactionLockParams{CharacterID: characterID}
		action := "faction.unlock"
		if reason != "" {
			action = "faction.lock"
			p.LockReason, p.LockedAt = text(reason), &now
			if hours > 0 {
				until := now.Add(time.Duration(hours) * time.Hour)
				p.LockedUntil = &until
			}
		}
		if err := q.SetFactionLock(ctx, p); err != nil {
			return err
		}
		after, err := q.GetCharacterFaction(ctx, characterID)
		if err != nil {
			return err
		}
		out = lockView(after)
		b, _ := json.Marshal(before)
		a, _ := json.Marshal(out)
		return q.InsertAdminAudit(ctx, db.InsertAdminAuditParams{AdminUserID: admin, Action: action, TargetType: "character",
			TargetID: characterID.String(), Before: b, After: a, Reason: pgtype.Text{String: note, Valid: true}})
	})
	return out, err
}

// ───────────────────────────── helpers ─────────────────────────────

// enqueueLeaderboard mirrors cultivation.EnqueueLeaderboard (same topic and payload).
func enqueueLeaderboard(ctx context.Context, q *db.Queries, characterID uuid.UUID, score float64) error {
	payload, _ := json.Marshal(map[string]any{"characterId": characterID, "score": score})
	return q.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{Topic: "leaderboard.dirty", AggregateID: characterID.String(), Payload: payload})
}

func enqueueChanged(ctx context.Context, q *db.Queries, e ChangedEvent) error {
	payload, _ := json.Marshal(e)
	return q.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{Topic: TopicChanged, AggregateID: e.CharacterID.String(), Payload: payload})
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func ptr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	v := t.String
	return &v
}

func date(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

// ───────────────────────────── GET /factions ─────────────────────────────

// FactionView is one faction of GET /factions.
type FactionView struct {
	Faction
	Open       bool  `json:"open"`       // joinable in the current era
	Members    int64 `json:"members"`    // member count
	MeritMilli int64 `json:"meritMilli"` // Σ credited points of the era (milli)
	MeritRank  int   `json:"meritRank"`  // 1-based among open or populated factions; 0 = unranked
}

// ListView is GET /factions.
type ListView struct {
	Factions              []FactionView `json:"factions"`
	SwitchCooldownSeconds int64         `json:"switchCooldownSeconds"`
	EraID                 string        `json:"eraId"`
	MeritSince            *string       `json:"meritSince"`       // first game day counted (null before launch)
	RankingAvailable      bool          `json:"rankingAvailable"` // false → counts from Postgres, merit unknown (0)
}

// List returns the catalogue with open flags, member counts and merit ranking (Redis; Postgres counts as fallback).
func (s *Service) List(ctx context.Context) (ListView, error) {
	q := db.New(s.Pool)
	ws, err := q.GetWorldState(ctx)
	if err != nil {
		return ListView{}, fmt.Errorf("world state: %w", err)
	}
	cur := s.eraSeq(ws)
	v := ListView{SwitchCooldownSeconds: int64(s.Cat.Settings.SwitchCooldown / time.Second), Factions: make([]FactionView, 0, len(s.Cat.Factions))}
	if len(s.World.Eras) > cur {
		v.EraID = s.World.Eras[cur].ID
	}
	if since := MeritSince(ws); !since.IsZero() {
		d := since.Format(time.DateOnly)
		v.MeritSince = &d
	}
	var members, merit map[string]int64
	if s.Ranking != nil {
		if members, merit, err = s.Ranking.Board(ctx); err == nil {
			v.RankingAvailable = true
		}
	}
	if !v.RankingAvailable {
		if members, err = s.fallbackCounts(ctx, q); err != nil {
			return ListView{}, err
		}
	}
	for _, f := range s.Cat.Factions {
		v.Factions = append(v.Factions, FactionView{Faction: f, Open: f.OpenIn(s.World, cur), Members: members[f.ID], MeritMilli: merit[f.ID]})
	}
	order := make([]int, 0, len(v.Factions))
	for i, f := range v.Factions {
		if f.Open || f.Members > 0 {
			order = append(order, i)
		}
	}
	slices.SortStableFunc(order, func(a, b int) int { return int(v.Factions[b].MeritMilli - v.Factions[a].MeritMilli) })
	for rank, i := range order {
		v.Factions[i].MeritRank = rank + 1
	}
	return v, nil
}

// fallbackCounts reads member counts from Postgres when Redis is down, cached in-process for fallbackTTL (L7).
func (s *Service) fallbackCounts(ctx context.Context, q *db.Queries) (map[string]int64, error) {
	s.fallbackMu.Lock()
	defer s.fallbackMu.Unlock()
	now := s.Now()
	if s.fallbackMembers != nil && now.Sub(s.fallbackAt) < fallbackTTL {
		return s.fallbackMembers, nil
	}
	rows, err := q.CountFactionMembers(ctx)
	if err != nil {
		return nil, err
	}
	members := make(map[string]int64, len(rows))
	for _, r := range rows {
		members[r.FactionID] = r.Members
	}
	s.fallbackMembers, s.fallbackAt = members, now
	return members, nil
}

// MemberEntry is one row of GET /factions/{id}/members.
type MemberEntry struct {
	Rank        int64     `json:"rank"`
	CharacterID uuid.UUID `json:"characterId"`
	Name        string    `json:"name"`
	StageID     string    `json:"stageId"`
	// Equipped cosmetics (display only, never ranking input).
	AvatarFrameID string `json:"avatarFrameId,omitempty"`
	TitleID       string `json:"titleId,omitempty"`
	// Chosen look resolved for the member's path (appearances.yaml). Display only.
	Look *appearance.Look `json:"look,omitempty"`
}

// MembersPage is GET /factions/{id}/members.
type MembersPage struct {
	FactionID  string        `json:"factionId"`
	Entries    []MemberEntry `json:"entries"`
	Total      int64         `json:"total"`
	NextCursor *string       `json:"nextCursor"` // opaque; null = last page
}

// MembersPageSize is the page size of the members list.
const MembersPageSize = 50

// Members pages a faction's members by realm score. cursor = "" or a previous nextCursor.
func (s *Service) Members(ctx context.Context, factionID, cursor string) (MembersPage, error) {
	if _, ok := s.Cat.Get(factionID); !ok {
		return MembersPage{}, ErrUnknownFaction
	}
	if s.Ranking == nil {
		return MembersPage{}, ErrRankingUnavailable
	}
	var offset int64
	if cursor != "" {
		n, err := strconv.ParseInt(cursor, 10, 64)
		if err != nil || n < 0 || n > 1_000_000 {
			return MembersPage{}, ErrBadCursor
		}
		offset = n
	}
	entries, total, err := s.Ranking.Members(ctx, factionID, offset, MembersPageSize)
	if err != nil {
		return MembersPage{}, err
	}
	out := MembersPage{FactionID: factionID, Entries: make([]MemberEntry, 0, len(entries)), Total: total}
	for _, e := range entries {
		out.Entries = append(out.Entries, MemberEntry{Rank: e.Rank, CharacterID: e.CharacterID, Name: e.Name, StageID: e.StageID,
			AvatarFrameID: e.AvatarFrameID, TitleID: e.TitleID, Look: e.Look})
	}
	if next := offset + MembersPageSize; next < total {
		c := strconv.FormatInt(next, 10)
		out.NextCursor = &c
	}
	return out, nil
}

// ErrBadCursor is an invalid members cursor.
var ErrBadCursor = errors.New("faction: invalid cursor")

// upcomingInto lists the branches of a scheduled split still open in era cur (nil for a migrate).
func (s *Service) upcomingInto(eff world.Effect, cur int) []string {
	if eff.Kind != world.EffectSplitFaction {
		return eff.Into
	}
	return s.Cat.OpenBranches(s.World, eff.Into, cur)
}
