package protector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/db"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/netsig"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/realm"
)

// Domain errors (mapped to HTTP in httpapi).
var (
	ErrNoCharacter  = errors.New("protector: character not found")
	ErrNoFaction    = errors.New("protector: not in a faction")
	ErrNotEligible  = errors.New("protector: not in front of a tribulation")
	ErrInviteGone   = errors.New("protector: invite expired, used or not found")
	ErrOtherFaction = errors.New("protector: invite belongs to another faction")
	ErrSelf         = errors.New("protector: cannot protect yourself")
	ErrDailyLimit   = errors.New("protector: daily assists used up")
	ErrKeyReused    = errors.New("protector: idempotency key reused for another invite")
	ErrNoPresence   = errors.New("protector: presence not configured")
	ErrNotReady     = errors.New("protector: the tribulation is not ready (cooldown, trial, cultivation…)")
	ErrTooLowRealm  = errors.New("protector: realm below protector.yaml min_tier")
	ErrTooNew       = errors.New("protector: character younger than protector.yaml min_character_age_days")
)

// TopicAssistCounted is the outbox topic of one counted protector (worker credits the protector_assist điểm duyên).
const TopicAssistCounted = "protector.assist_counted"

// AssistCountedEvent is its payload.
type AssistCountedEvent struct {
	AssistID             int64     `json:"assistId"`
	InviteID             string    `json:"inviteId"`
	ProtectorCharacterID string    `json:"protectorCharacterId"`
	ProtectorUserID      string    `json:"protectorUserId"`
	Ref                  string    `json:"ref"` // bt:<breakthrough key>
	OccurredAt           time.Time `json:"occurredAt"`
}

// ProtectorView is one accepted protector of an invite.
type ProtectorView struct {
	CharacterID string    `json:"characterId"`
	Name        string    `json:"name"`
	AcceptedAt  time.Time `json:"acceptedAt"`
	Online      bool      `json:"online"`
	SameFaction bool      `json:"sameFaction"`
	Counts      bool      `json:"counts"` // would count if the attempt happened now
}

// InviteView is the viewer's own open invite.
type InviteView struct {
	InviteID   uuid.UUID       `json:"inviteId"`
	StageID    string          `json:"stageId"`
	CreatedAt  time.Time       `json:"createdAt"`
	ExpiresAt  time.Time       `json:"expiresAt"`
	Protectors []ProtectorView `json:"protectors"`
	Counted    int             `json:"counted"` // protectors that count now (≤ MaxCounted)
}

// OpenInvite is a fellow member's invite the viewer may accept.
type OpenInvite struct {
	InviteID    uuid.UUID `json:"inviteId"`
	CharacterID string    `json:"characterId"`
	Name        string    `json:"name"`
	StageID     string    `json:"stageId"`
	ExpiresAt   time.Time `json:"expiresAt"`
	Protectors  int       `json:"protectors"`
	Accepted    bool      `json:"accepted"` // the viewer already guards it
}

// Member is an online fellow member.
type Member struct {
	CharacterID string `json:"characterId"`
	Name        string `json:"name"`
	StageID     string `json:"stageId"`
}

// View is GET /protector and the answer to every protector mutation.
type View struct {
	ServerNow           time.Time    `json:"serverNow"`
	FactionID           string       `json:"factionId"`
	Eligible            bool         `json:"eligible"` // the viewer stands before a tribulation (may invite)
	PingEverySeconds    int64        `json:"pingEverySeconds"`
	InviteWindowSeconds int64        `json:"inviteWindowSeconds"`
	DailyAssists        int32        `json:"dailyAssists"`
	AssistsLeft         int32        `json:"assistsLeft"`
	MaxCounted          int          `json:"maxCounted"`
	PerMemberBP         int64        `json:"perMemberBp"`
	MaxBP               int64        `json:"maxBp"`
	MyInvite            *InviteView  `json:"myInvite"`
	OpenInvites         []OpenInvite `json:"openInvites"`
	OnlineMembers       []Member     `json:"onlineMembers"`
	Online              bool         `json:"online"` // the viewer's own presence
}

// Service owns hộ pháp.
type Service struct {
	Pool     *pgxpool.Pool
	Cfg      *Config
	Realms   *realm.Catalogue
	Presence Presence // nil → nobody is online (no bonus), invite/accept still work
	// Project returns the character's stage after the pending idle settlement, read-only.
	Project func(ctx context.Context, q *db.Queries, userID uuid.UUID, p db.CharacterProgression) (*realm.Stage, error)
	// Ready reports whether the user's tribulation could be attempted now (breakthrough preview Ready + tribulation).
	// Injected (breakthrough imports this package). nil → only the stage is checked.
	Ready func(ctx context.Context, userID uuid.UUID) (bool, error)
	Now   func() time.Time
	Log   *slog.Logger
	// Net flags inviter/protector pairs whose accounts shared a network recently (admin only, never blocks). nil → off.
	Net *netsig.Store
}

func (s *Service) maxCounted() int {
	t := s.Realms.Tribulation()
	return MaxCounted(t.ProtectorPerBP, t.ProtectorMaxBP)
}

// eligibleStage: on the last stage of a realm whose major breakthrough is a tribulation.
func eligibleStage(cat *realm.Catalogue, st *realm.Stage) bool {
	if st == nil || !st.IsLastOfRealm() || !st.Tribulation {
		return false
	}
	return cat.Next(st) != nil
}

func (s *Service) faction(ctx context.Context, q *db.Queries, charID uuid.UUID) (string, error) {
	cf, err := q.GetCharacterFaction(ctx, charID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return cf.FactionID, nil
}

// Ping marks the viewer online (Redis TTL). Read-only for Postgres.
func (s *Service) Ping(ctx context.Context, userID uuid.UUID) (bool, error) {
	if s.Presence == nil {
		return false, ErrNoPresence
	}
	q := db.New(s.Pool)
	ch, err := q.GetCharacterByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNoCharacter
	} else if err != nil {
		return false, err
	}
	fid, err := s.faction(ctx, q, ch.ID)
	if err != nil {
		return false, err
	}
	return s.Presence.Ping(ctx, ch.ID, userID, fid, s.Now().UTC())
}

// touch refreshes the caller's presence after an invite/accept (best effort). It does NOT mark the user active: only the
// httpapi allowlist (markActive) does, so the ping counts only if a real game action happened within the window.
func (s *Service) touch(ctx context.Context, userID, charID uuid.UUID) {
	if s.Presence == nil {
		return
	}
	if fid, err := s.faction(ctx, db.New(s.Pool), charID); err == nil {
		_, _ = s.Presence.Ping(ctx, charID, userID, fid, s.Now().UTC())
	}
}

// online reads presence for ids (outside any transaction). Presence errors degrade to "nobody online".
func (s *Service) online(ctx context.Context, ids []uuid.UUID, now time.Time) map[uuid.UUID]bool {
	if s.Presence == nil || len(ids) == 0 {
		return map[uuid.UUID]bool{}
	}
	m, err := s.Presence.Online(ctx, ids, now)
	if err != nil {
		if s.Log != nil {
			s.Log.WarnContext(ctx, "presence read", "err", err)
		}
		return map[uuid.UUID]bool{}
	}
	return m
}

// State is GET /protector.
func (s *Service) State(ctx context.Context, userID uuid.UUID) (View, error) {
	q := db.New(s.Pool)
	ch, err := q.GetCharacterByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return View{}, ErrNoCharacter
	} else if err != nil {
		return View{}, err
	}
	return s.view(ctx, q, userID, ch)
}

func (s *Service) view(ctx context.Context, q *db.Queries, userID uuid.UUID, ch db.Character) (View, error) {
	now := s.Now().UTC()
	t := s.Realms.Tribulation()
	v := View{ServerNow: now, PingEverySeconds: s.Cfg.PingEverySeconds, InviteWindowSeconds: s.Cfg.InviteWindowSeconds,
		DailyAssists: s.Cfg.DailyAssists, MaxCounted: s.maxCounted(), PerMemberBP: t.ProtectorPerBP, MaxBP: t.ProtectorMaxBP,
		OpenInvites: []OpenInvite{}, OnlineMembers: []Member{}}
	fid, err := s.faction(ctx, q, ch.ID)
	if err != nil {
		return v, err
	}
	v.FactionID = fid
	p, err := q.GetProgression(ctx, ch.ID)
	if err != nil {
		return v, err
	}
	stage, err := s.Project(ctx, q, userID, p)
	if err != nil {
		return v, err
	}
	v.Eligible = fid != "" && eligibleStage(s.Realms, stage)
	uses, err := q.CountProtectorUses(ctx, db.CountProtectorUsesParams{ProtectorCharacterID: ch.ID, GameDay: gameday.PG(now), Now: now})
	if err != nil {
		return v, err
	}
	v.AssistsLeft = max(0, s.Cfg.DailyAssists-uses)
	v.Online = s.online(ctx, []uuid.UUID{ch.ID}, now)[ch.ID]
	if inv, err := q.GetOpenProtectorInvite(ctx, db.GetOpenProtectorInviteParams{CharacterID: ch.ID, Now: now}); err == nil {
		iv, err := s.inviteView(ctx, q, inv, fid, now)
		if err != nil {
			return v, err
		}
		v.MyInvite = &iv
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return v, err
	}
	if fid == "" {
		return v, nil
	}
	mine, err := q.ListMyAcceptedInvites(ctx, db.ListMyAcceptedInvitesParams{ProtectorCharacterID: ch.ID, Now: now})
	if err != nil {
		return v, err
	}
	accepted := map[uuid.UUID]bool{}
	for _, id := range mine {
		accepted[id] = true
	}
	open, err := q.ListFactionOpenInvites(ctx, db.ListFactionOpenInvitesParams{FactionID: fid, Now: now, Lim: s.Cfg.ListLimit})
	if err != nil {
		return v, err
	}
	for _, o := range open {
		if o.CharacterID == ch.ID {
			continue
		}
		v.OpenInvites = append(v.OpenInvites, OpenInvite{InviteID: o.ID, CharacterID: o.CharacterID.String(), Name: o.Name, StageID: o.StageID,
			ExpiresAt: o.ExpiresAt, Protectors: int(o.Protectors), Accepted: accepted[o.ID]})
	}
	if s.Presence != nil {
		ids, err := s.Presence.FactionOnline(ctx, fid, now, int64(s.Cfg.ListLimit)+1)
		if err != nil {
			if s.Log != nil {
				s.Log.WarnContext(ctx, "presence faction read", "err", err)
			}
		} else if len(ids) > 0 {
			briefs, err := q.ListCharacterBriefs(ctx, ids)
			if err != nil {
				return v, err
			}
			byID := map[uuid.UUID]db.ListCharacterBriefsRow{}
			for _, b := range briefs {
				byID[b.ID] = b
			}
			for _, id := range ids {
				if b, ok := byID[id]; ok && id != ch.ID && len(v.OnlineMembers) < int(s.Cfg.ListLimit) {
					v.OnlineMembers = append(v.OnlineMembers, Member{CharacterID: id.String(), Name: b.Name, StageID: b.StageID})
				}
			}
		}
	}
	return v, nil
}

// candidates lists an invite's accepted protectors with presence (presence read outside the transaction — callers
// pass the map they read before their transaction when counting for real).
func (s *Service) candidates(ctx context.Context, q *db.Queries, inviteID uuid.UUID) ([]db.ListInviteProtectorsRow, error) {
	return q.ListInviteProtectors(ctx, inviteID)
}

func toCandidates(rows []db.ListInviteProtectorsRow, online map[uuid.UUID]bool) []Candidate {
	out := make([]Candidate, 0, len(rows))
	for _, r := range rows {
		out = append(out, Candidate{AssistID: r.ID, CharacterID: r.ProtectorCharacterID.String(), FactionID: r.FactionID.String,
			Online: online[r.ProtectorCharacterID]})
	}
	return out
}

func (s *Service) inviteView(ctx context.Context, q *db.Queries, inv db.ProtectorInvite, inviterFaction string, now time.Time) (InviteView, error) {
	rows, err := s.candidates(ctx, q, inv.ID)
	if err != nil {
		return InviteView{}, err
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ProtectorCharacterID
	}
	online := s.online(ctx, ids, now)
	fid := inv.FactionID
	if inviterFaction != fid {
		fid = "" // the inviter left the faction: nobody counts
	}
	counted := map[int64]bool{}
	for _, c := range Count(toCandidates(rows, online), fid, s.maxCounted()) {
		counted[c.AssistID] = true
	}
	iv := InviteView{InviteID: inv.ID, StageID: inv.StageID, CreatedAt: inv.CreatedAt, ExpiresAt: inv.ExpiresAt, Protectors: []ProtectorView{},
		Counted: len(counted)}
	for _, r := range rows {
		iv.Protectors = append(iv.Protectors, ProtectorView{CharacterID: r.ProtectorCharacterID.String(), Name: r.Name, AcceptedAt: r.AcceptedAt,
			Online: online[r.ProtectorCharacterID], SameFaction: r.FactionID.String == inv.FactionID, Counts: counted[r.ID]})
	}
	return iv, nil
}

// Invite opens (or returns) the viewer's invite. Under the progression lock (serializes with the tribulation attempt):
// idempotency → open invite exists → faction + eligibility (projected stage) → insert.
func (s *Service) Invite(ctx context.Context, userID uuid.UUID, idemKey string) (View, error) {
	var ch db.Character
	// L2: only a tribulation that could be attempted now (no cooldown, trial, cultivation or item missing). Read before
	// the tx (the preview is read-only); a race only means an invite that expires unused — protectors are not charged.
	ready := true
	if s.Ready != nil {
		var err error
		if ready, err = s.Ready(ctx, userID); err != nil {
			return View{}, err
		}
	}
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		var err error
		ch, err = q.GetCharacterByUser(ctx, userID)
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
		if _, err := q.GetProtectorInviteByKey(ctx, db.GetProtectorInviteByKeyParams{CharacterID: ch.ID, IdempotencyKey: idemKey}); err == nil {
			return nil // replay
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err := q.GetOpenProtectorInvite(ctx, db.GetOpenProtectorInviteParams{CharacterID: ch.ID, Now: now}); err == nil {
			return nil // one open invite at a time
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		fid, err := s.faction(ctx, q, ch.ID)
		if err != nil {
			return err
		}
		if fid == "" {
			return ErrNoFaction
		}
		stage, err := s.Project(ctx, q, userID, p)
		if err != nil {
			return err
		}
		if !eligibleStage(s.Realms, stage) {
			return ErrNotEligible
		}
		if !ready {
			return ErrNotReady
		}
		_, err = q.InsertProtectorInvite(ctx, db.InsertProtectorInviteParams{CharacterID: ch.ID, FactionID: fid, StageID: stage.ID,
			IdempotencyKey: idemKey, CreatedAt: now, ExpiresAt: now.Add(s.Cfg.InviteWindow())})
		return err
	})
	if err != nil {
		return View{}, err
	}
	s.touch(ctx, userID, ch.ID) // presence refresh only (best effort, after commit)
	return s.view(ctx, db.New(s.Pool), userID, ch)
}

// Accept answers a fellow member's invite. Lock order: the invite (FOR UPDATE, serializes with the attempt that uses
// it) → protector_days (the protector's day row = the daily limit's lock) → insert the assist. Idempotent by key;
// accepting the same invite twice returns the current state without spending a second use. A "use" is an assist of
// that day that was counted or is still pending on an open invite (L2: uncounted / expired ones are never charged).
// The protector must be ≥ min_tier and their character ≥ min_character_age_days old (M2).
func (s *Service) Accept(ctx context.Context, userID, inviteID uuid.UUID, idemKey string) (View, error) {
	var ch db.Character
	var inviter uuid.UUID
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		var err error
		ch, err = q.GetCharacterByUser(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoCharacter
		} else if err != nil {
			return err
		}
		now := s.Now().UTC()
		if a, err := q.GetProtectorAssistByKey(ctx, db.GetProtectorAssistByKeyParams{ProtectorCharacterID: ch.ID, IdempotencyKey: idemKey}); err == nil {
			if a.InviteID != inviteID {
				return ErrKeyReused
			}
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		inv, err := q.GetProtectorInviteForUpdate(ctx, inviteID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInviteGone
		} else if err != nil {
			return err
		}
		if inv.CharacterID == ch.ID {
			return ErrSelf
		}
		inviter = inv.CharacterID
		if inv.UsedAt != nil || !now.Before(inv.ExpiresAt) {
			return ErrInviteGone
		}
		fid, err := s.faction(ctx, q, ch.ID)
		if err != nil {
			return err
		}
		if fid == "" || fid != inv.FactionID {
			return ErrOtherFaction
		}
		created, err := q.GetCharacterCreatedAt(ctx, ch.ID)
		if err != nil {
			return err
		}
		if now.Sub(created) < s.Cfg.MinCharacterAge() {
			return ErrTooNew
		}
		p, err := q.GetProgression(ctx, ch.ID)
		if err != nil {
			return err
		}
		if st, err := s.Project(ctx, q, userID, p); err != nil {
			return err
		} else if st.Tier < s.Cfg.MinTier {
			return ErrTooLowRealm
		}
		if _, err := q.GetProtectorAssist(ctx, db.GetProtectorAssistParams{InviteID: inv.ID, ProtectorCharacterID: ch.ID}); err == nil {
			return nil // already guarding this invite
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		day := gameday.PG(now)
		if _, err := q.LockProtectorDay(ctx, db.LockProtectorDayParams{CharacterID: ch.ID, GameDay: day}); err != nil {
			return err
		}
		uses, err := q.CountProtectorUses(ctx, db.CountProtectorUsesParams{ProtectorCharacterID: ch.ID, GameDay: day, Now: now})
		if err != nil {
			return err
		}
		if uses >= s.Cfg.DailyAssists {
			return ErrDailyLimit
		}
		if _, err = q.InsertProtectorAssist(ctx, db.InsertProtectorAssistParams{InviteID: inv.ID, ProtectorCharacterID: ch.ID, GameDay: day,
			IdempotencyKey: idemKey, AcceptedAt: now}); err != nil {
			return err
		}
		return q.SetProtectorDayUses(ctx, db.SetProtectorDayUsesParams{CharacterID: ch.ID, GameDay: day, Uses: uses + 1})
	})
	if err != nil {
		return View{}, err
	}
	s.touch(ctx, userID, ch.ID) // presence refresh only (best effort, after commit)
	if shared, err := s.Net.SharedRecentlyCharacters(ctx, inviter, ch.ID); err == nil && shared && s.Log != nil {
		s.Log.WarnContext(ctx, "hộ pháp between accounts on the same network (admin review)", "inviter", inviter, "protector", ch.ID)
	}
	return s.view(ctx, db.New(s.Pool), userID, ch)
}

// Pair is one inviter → protector pair for admin review (anti-cheat M2).
type Pair struct {
	InviterID     string `json:"inviterId"`
	InviterName   string `json:"inviterName"`
	ProtectorID   string `json:"protectorId"`
	ProtectorName string `json:"protectorName"`
	Assists       int32  `json:"assists"`
	Counted       int32  `json:"counted"`
	FirstDay      string `json:"firstDay"`
	LastDay       string `json:"lastDay"`
	SharedNetwork bool   `json:"sharedNetwork"` // the two accounts shared a network hash within 30 days (netsig; flag only)
}

// Pairs lists pairs with at least minAssists assists over the last `days` game days (most frequent first, ≤ 200).
func (s *Service) Pairs(ctx context.Context, minAssists, days int) ([]Pair, error) {
	since := gameday.Of(s.Now().UTC()).AddDate(0, 0, -max(0, days))
	rows, err := db.New(s.Pool).ListProtectorPairs(ctx, db.ListProtectorPairsParams{Since: pgtype.Date{Time: since, Valid: true},
		MinAssists: int64(max(1, minAssists)), Lim: 200})
	if err != nil {
		return nil, err
	}
	out := make([]Pair, 0, len(rows))
	for _, r := range rows {
		out = append(out, Pair{InviterID: r.InviterID.String(), InviterName: r.InviterName, ProtectorID: r.ProtectorID.String(),
			ProtectorName: r.ProtectorName, Assists: r.Assists, Counted: r.Counted, FirstDay: r.FirstDay.Time.Format(time.DateOnly),
			LastDay: r.LastDay.Time.Format(time.DateOnly)})
		if shared, err := s.Net.SharedRecentlyCharacters(ctx, r.InviterID, r.ProtectorID); err == nil {
			out[len(out)-1].SharedNetwork = shared
		}
	}
	return out, nil
}

// ───── used by the breakthrough ─────

// Snapshot is the presence read the breakthrough takes BEFORE its transaction (no Redis call inside a tx).
type Snapshot struct {
	Online map[uuid.UUID]bool
}

// Prepare reads the candidates of the character's open invite (no lock) and their presence. Safe with a nil Service.
func (s *Service) Prepare(ctx context.Context, q *db.Queries, charID uuid.UUID) Snapshot {
	snap := Snapshot{Online: map[uuid.UUID]bool{}}
	if s == nil {
		return snap
	}
	now := s.Now().UTC()
	inv, err := q.GetOpenProtectorInvite(ctx, db.GetOpenProtectorInviteParams{CharacterID: charID, Now: now})
	if err != nil {
		return snap
	}
	rows, err := s.candidates(ctx, q, inv.ID)
	if err != nil {
		return snap
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ProtectorCharacterID
	}
	snap.Online = s.online(ctx, ids, now)
	return snap
}

// Counted is the protector part of one attempt.
type Counted struct {
	InviteID   uuid.UUID
	Protectors []Candidate
	rows       map[int64]db.ListInviteProtectorsRow
}

// N is the number of counted protectors.
func (c *Counted) N() int64 {
	if c == nil {
		return 0
	}
	return int64(len(c.Protectors))
}

// CountLocked counts the protectors of the character's open invite for a tribulation at stage, inside the attempt's
// transaction (lock = FOR UPDATE on the invite; call after the progression lock). The invite must be for this stage
// and the character still in the invite's faction. Presence comes from snap (read before the tx).
func (s *Service) CountLocked(ctx context.Context, q *db.Queries, charID uuid.UUID, stage *realm.Stage, snap Snapshot, now time.Time, lock bool) (*Counted, error) {
	if s == nil {
		return nil, nil
	}
	var inv db.ProtectorInvite
	var err error
	if lock {
		inv, err = q.GetOpenProtectorInviteForUpdate(ctx, db.GetOpenProtectorInviteForUpdateParams{CharacterID: charID, Now: now})
	} else {
		inv, err = q.GetOpenProtectorInvite(ctx, db.GetOpenProtectorInviteParams{CharacterID: charID, Now: now})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("protector invite: %w", err)
	}
	if inv.StageID != stage.ID {
		return nil, nil
	}
	fid, err := s.faction(ctx, q, charID)
	if err != nil {
		return nil, err
	}
	c := &Counted{InviteID: inv.ID, Protectors: []Candidate{}, rows: map[int64]db.ListInviteProtectorsRow{}}
	if fid != inv.FactionID {
		return c, nil
	}
	rows, err := s.candidates(ctx, q, inv.ID)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		c.rows[r.ID] = r
	}
	c.Protectors = Count(toCandidates(rows, snap.Online), fid, s.maxCounted())
	return c, nil
}

// UseLocked consumes the invite for the attempt (ref = "bt:<key>"), stamps the counted assists and enqueues one
// outbox event per counted protector (the worker credits their protector_assist điểm duyên — never inside this tx,
// which would lock other players' vip_accounts and could deadlock with their own attempt).
func (s *Service) UseLocked(ctx context.Context, q *db.Queries, c *Counted, ref string, now time.Time) error {
	if s == nil || c == nil {
		return nil
	}
	if _, err := q.UseProtectorInvite(ctx, db.UseProtectorInviteParams{ID: c.InviteID, UsedAt: &now, UsedRef: pgtype.Text{String: ref, Valid: true}}); err != nil {
		return fmt.Errorf("use invite: %w", err)
	}
	for _, p := range c.Protectors {
		if err := q.CountProtectorAssist(ctx, db.CountProtectorAssistParams{ID: p.AssistID, CountedAt: &now}); err != nil {
			return err
		}
		r := c.rows[p.AssistID]
		payload, _ := json.Marshal(AssistCountedEvent{AssistID: p.AssistID, InviteID: c.InviteID.String(), ProtectorCharacterID: p.CharacterID,
			ProtectorUserID: r.UserID.String(), Ref: ref, OccurredAt: now})
		if err := q.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{Topic: TopicAssistCounted, AggregateID: p.CharacterID, Payload: payload}); err != nil {
			return err
		}
	}
	return nil
}
