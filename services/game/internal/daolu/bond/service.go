// Package bond runs đạo lữ at runtime: the bond flow (propose → accept with the double "vĩnh viễn" confirmation,
// endings) and song tu (Redis overlap session + claims through cultivation.CreditLocked). Catalogue and pure rules live
// in package daolu (imported by content.LoadAll, so this package may depend on cultivation).
package bond

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/daolu"
	"tiendao/game/internal/db"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/netsig"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/protector"
	"tiendao/game/internal/realm"
	"tiendao/game/internal/vip"
)

// Domain errors (mapped to HTTP in httpapi).
var (
	ErrNoCharacter      = errors.New("daolu: character not found")
	ErrTargetNotFound   = errors.New("daolu: target character not found")
	ErrSelf             = errors.New("daolu: cannot propose to yourself")
	ErrNotAcknowledged  = errors.New("daolu: the permanent-bond warning must be acknowledged")
	ErrOpenProposal     = errors.New("daolu: you already have an open proposal")
	ErrDailyLimit       = errors.New("daolu: daily proposal limit reached")
	ErrProposalGone     = errors.New("daolu: proposal expired, answered or not found")
	ErrKeyReused        = errors.New("daolu: idempotency key reused for another request")
	ErrAlreadyBonded    = errors.New("daolu: one of you is already bonded")
	ErrNoBond           = errors.New("daolu: no active bond")
	ErrBondEnded        = errors.New("daolu: bond already ended")
	ErrBadReason        = errors.New("daolu: unknown end reason")
	ErrReportMismatch   = errors.New("daolu: the report does not concern either partner")
	ErrNoteRequired     = errors.New("daolu: a note is required")
	ErrSongTuOff        = errors.New("daolu: song tu not configured on this server (needs Redis)")
	ErrNothingToClaim   = errors.New("daolu: no unclaimed song tu time")
	ErrSongTuDayCapped  = errors.New("daolu: today's song tu window is already fully paid")
	ErrNotYourProposal  = errors.New("daolu: not your proposal")
	ErrDeclinedRecently = errors.New("daolu: this character declined you recently")
	errUniqueActiveBond = "dao_lu_members_active_uq"
)

// GateError lists why one side cannot bond (dao_lu.yaml gates). Side is "self" or "target".
type GateError struct {
	Side     string
	Blockers []string
}

func (e *GateError) Error() string {
	return fmt.Sprintf("daolu: %s cannot bond now: %s", e.Side, strings.Join(e.Blockers, ","))
}

// End reasons (dao_lu_bonds.end_reason).
const (
	ReasonLuanHoi   = "luan_hoi"   // luân hồi (G4, Phase 6) — call EndForCharacterLocked
	ReasonPhongThan = "phong_than" // lên Phong Thần Bảng / luân hồi in Phong Thần Lượng Kiếp (Phase 8)
	ReasonAdmin     = "admin"      // admin after a report (audited)
	ReasonAbsence   = "absence"    // absence_days without play (worker)
)

// Service owns the bond flow and song tu.
type Service struct {
	Pool        *pgxpool.Pool
	Cfg         *daolu.Config
	Realms      *realm.Catalogue
	Cultivation *cultivation.Service // the single credit path (song tu claim)
	VIP         *vip.Table           // dual_cultivation điểm duyên
	Session     *Session             // nil → song tu unavailable (no Redis)
	Presence    protector.Presence   // partner "online" on the bond card; nil → offline
	Net         *netsig.Store        // shared-network signal; nil → never flagged
	Now         func() time.Time
	Log         *slog.Logger
}

// RulesView is the catalogue part the client shows (numbers never hardcoded in TS).
type RulesView struct {
	MinAccountAgeDays   int64 `json:"minAccountAgeDays"`
	MinTier             int   `json:"minTier"`
	ProposalExpiryHours int64 `json:"proposalExpiryHours"`
	DailyProposals      int32 `json:"dailyProposals"`
	AbsenceDays         int64 `json:"absenceDays"`
	RebondCooldownDays  int64 `json:"rebondCooldownDays"`
	WindowSeconds       int64 `json:"windowSeconds"`
	RateMilliPerHour    int64 `json:"rateMilliPerHour"`
	HeartbeatSeconds    int64 `json:"heartbeatSeconds"`
	MaxSongTuMilli      int64 `json:"maxSongTuMilli"`
	DuyenMinSeconds     int64 `json:"duyenMinSeconds"`
}

// PartnerView is the other side of a bond.
type PartnerView struct {
	CharacterID string `json:"characterId"`
	Name        string `json:"name"`
	StageID     string `json:"stageId"`
	Online      bool   `json:"online"`
}

// BondView is the viewer's active bond.
type BondView struct {
	BondID   uuid.UUID   `json:"bondId"`
	Partner  PartnerView `json:"partner"`
	FormedAt time.Time   `json:"formedAt"`
}

// ProposalView is one open proposal (incoming: from CharacterID; outgoing: to CharacterID).
type ProposalView struct {
	ProposalID  uuid.UUID `json:"proposalId"`
	CharacterID string    `json:"characterId"`
	Name        string    `json:"name"`
	StageID     string    `json:"stageId"`
	CreatedAt   time.Time `json:"createdAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

// SongTuView is today's song tu state for the viewer.
type SongTuView struct {
	Available        bool       `json:"available"` // Redis configured
	GameDay          string     `json:"gameDay"`
	ResetAt          time.Time  `json:"resetAt"` // unclaimed overlap is lost at the game-day reset
	SessionStarted   bool       `json:"sessionStarted"`
	AccruedSeconds   int64      `json:"accruedSeconds"`   // shared overlap today
	PaidSeconds      int64      `json:"paidSeconds"`      // the viewer's paid seconds today (DB)
	ClaimableSeconds int64      `json:"claimableSeconds"` // unclaimed, within the window left
	WindowSeconds    int64      `json:"windowSeconds"`
	MeActive         bool       `json:"meActive"`
	PartnerActive    bool       `json:"partnerActive"`
	PartnerBeatAt    *time.Time `json:"partnerBeatAt"`
	HeartbeatSeconds int64      `json:"heartbeatSeconds"`
	ClaimableMilli   int64      `json:"claimableMilli"` // raw tu vi of ClaimableSeconds (caps may clip it)
}

// View is GET /dao-lu and the answer to every bond mutation.
type View struct {
	ServerNow     time.Time      `json:"serverNow"`
	Rules         RulesView      `json:"rules"`
	Blockers      []string       `json:"blockers"` // why the viewer cannot propose/accept now
	CooldownUntil *time.Time     `json:"cooldownUntil"`
	ProposalsLeft int32          `json:"proposalsLeft"`
	Bond          *BondView      `json:"bond"`
	Incoming      []ProposalView `json:"incoming"`
	Outgoing      []ProposalView `json:"outgoing"`
	SongTu        *SongTuView    `json:"songTu"`
}

func (s *Service) rules() RulesView {
	c := s.Cfg
	return RulesView{MinAccountAgeDays: c.Bond.MinAccountAgeDays, MinTier: c.Bond.MinTier, ProposalExpiryHours: c.Bond.ProposalExpiryHours,
		DailyProposals: c.Bond.DailyProposals, AbsenceDays: c.Bond.AbsenceDays, RebondCooldownDays: c.Bond.RebondCooldownDays,
		WindowSeconds: c.SongTu.WindowSeconds, RateMilliPerHour: c.SongTu.RateMilliPerHour, HeartbeatSeconds: c.SongTu.HeartbeatSeconds,
		MaxSongTuMilli: c.MaxSongTuMilli(), DuyenMinSeconds: c.SongTu.DuyenMinSeconds}
}

func (s *Service) me(ctx context.Context, q *db.Queries, userID uuid.UUID) (db.Character, error) {
	ch, err := q.GetCharacterByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ch, ErrNoCharacter
	}
	return ch, err
}

// party loads the gate inputs of one character.
func (s *Service) party(ctx context.Context, q *db.Queries, charID uuid.UUID) (db.GetDaoLuPartyRow, daolu.Party, error) {
	row, err := q.GetDaoLuParty(ctx, charID)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, daolu.Party{}, ErrTargetNotFound
	} else if err != nil {
		return row, daolu.Party{}, err
	}
	p := daolu.Party{AccountCreatedAt: row.AccountCreatedAt}
	if st, err := s.Realms.Stage(row.StageID); err == nil {
		p.Tier = st.Tier
	}
	if _, err := q.GetActiveDaoLuBond(ctx, charID); err == nil {
		p.Bonded = true
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return row, p, err
	}
	if p.LastEndedAt, err = q.LastDaoLuEndedAt(ctx, charID); err != nil {
		return row, p, err
	}
	return row, p, nil
}

// State is GET /dao-lu.
func (s *Service) State(ctx context.Context, userID uuid.UUID) (View, error) {
	q := db.New(s.Pool)
	ch, err := s.me(ctx, q, userID)
	if err != nil {
		return View{}, err
	}
	return s.view(ctx, q, userID, ch.ID)
}

func (s *Service) view(ctx context.Context, q *db.Queries, userID, charID uuid.UUID) (View, error) {
	now := s.Now().UTC()
	v := View{ServerNow: now, Rules: s.rules(), Blockers: []string{}, Incoming: []ProposalView{}, Outgoing: []ProposalView{}}
	_, me, err := s.party(ctx, q, charID)
	if err != nil {
		return v, err
	}
	v.Blockers = s.Cfg.Blockers(me, now)
	if u := s.Cfg.CooldownUntil(me.LastEndedAt, now); !u.IsZero() {
		v.CooldownUntil = &u
	}
	sent, err := q.CountDaoLuProposalsDay(ctx, db.CountDaoLuProposalsDayParams{CharacterID: charID, GameDay: gameday.PG(now)})
	if err != nil {
		return v, err
	}
	v.ProposalsLeft = max(0, s.Cfg.Bond.DailyProposals-sent)
	in, err := q.ListDaoLuIncoming(ctx, db.ListDaoLuIncomingParams{CharacterID: charID, Now: now, Lim: s.Cfg.Bond.IncomingListLimit})
	if err != nil {
		return v, err
	}
	for _, r := range in {
		v.Incoming = append(v.Incoming, ProposalView{ProposalID: r.ID, CharacterID: r.CharacterID.String(), Name: r.Name, StageID: r.StageID,
			CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt})
	}
	out, err := q.ListDaoLuOutgoing(ctx, db.ListDaoLuOutgoingParams{CharacterID: charID, Now: now, Lim: s.Cfg.Bond.IncomingListLimit})
	if err != nil {
		return v, err
	}
	for _, r := range out {
		v.Outgoing = append(v.Outgoing, ProposalView{ProposalID: r.ID, CharacterID: r.CharacterID.String(), Name: r.Name, StageID: r.StageID,
			CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt})
	}
	bond, err := q.GetActiveDaoLuBond(ctx, charID)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, nil
	} else if err != nil {
		return v, err
	}
	pair, partner, err := s.pairOf(ctx, q, userID, charID, bond)
	if err != nil {
		return v, err
	}
	bv := &BondView{BondID: bond.ID, FormedAt: bond.FormedAt, Partner: PartnerView{CharacterID: partner.ID.String(), Name: partner.Name, StageID: partner.StageID}}
	if s.Presence != nil {
		if m, err := s.Presence.Online(ctx, []uuid.UUID{partner.ID}, now); err == nil {
			bv.Partner.Online = m[partner.ID]
		}
	}
	v.Bond = bv
	st, err := s.songTuView(ctx, q, pair, nil, now)
	if err != nil {
		return v, err
	}
	v.SongTu = &st
	return v, nil
}

func (s *Service) pairOf(ctx context.Context, q *db.Queries, userID, charID uuid.UUID, bond db.DaoLuBond) (Pair, db.GetDaoLuPartyRow, error) {
	other := bond.CharacterA
	if other == charID {
		other = bond.CharacterB
	}
	partner, err := q.GetDaoLuParty(ctx, other)
	if err != nil {
		return Pair{}, partner, fmt.Errorf("partner: %w", err)
	}
	return Pair{BondID: bond.ID, MeChar: charID, MeUser: userID, PartnerChar: other, PartnerUser: partner.UserID}, partner, nil
}

func uuidPG(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func (s *Service) flagLog(ctx context.Context, msg string, a, b uuid.UUID) {
	if s.Log != nil {
		s.Log.WarnContext(ctx, msg, "characterA", a, "characterB", b)
	}
}

// Propose sends a proposal to target (from a player card). ack = the proposer confirmed the "vĩnh viễn" warning.
// Under the proposer's progression lock (serializes their proposals): idempotency → gates (both sides) → one open
// outgoing proposal → daily limit → insert (+ net_flag event when the two accounts shared a network recently).
func (s *Service) Propose(ctx context.Context, userID, target uuid.UUID, ack bool, idemKey string) (View, error) {
	if !ack {
		return View{}, ErrNotAcknowledged
	}
	q0 := db.New(s.Pool)
	me, err := s.me(ctx, q0, userID)
	if err != nil {
		return View{}, err
	}
	if me.ID == target {
		return View{}, ErrSelf
	}
	tparty, err := q0.GetDaoLuParty(ctx, target)
	if errors.Is(err, pgx.ErrNoRows) {
		return View{}, ErrTargetNotFound
	} else if err != nil {
		return View{}, err
	}
	shared, err := s.Net.SharedRecently(ctx, userID, tparty.UserID)
	if err != nil {
		return View{}, fmt.Errorf("net signal: %w", err)
	}
	err = postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		if _, err := q.GetProgressionForUpdate(ctx, me.ID); err != nil {
			return fmt.Errorf("lock progression: %w", err)
		}
		now := s.Now().UTC()
		if prev, err := q.GetDaoLuProposalByKey(ctx, db.GetDaoLuProposalByKeyParams{ProposerCharacterID: me.ID, IdempotencyKey: idemKey}); err == nil {
			if prev.TargetCharacterID != target {
				return ErrKeyReused
			}
			return nil // replay
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		// Review L1: lock both characters' pending rows in id order (the accept path's order), then one lazy-expiry update.
		ids := []uuid.UUID{me.ID, target}
		if _, err := q.LockDaoLuPendingFor(ctx, ids); err != nil {
			return err
		}
		if err := q.ExpireDaoLuProposals(ctx, db.ExpireDaoLuProposalsParams{Ids: ids, Now: now}); err != nil {
			return err
		}
		if err := s.gates(ctx, q, me.ID, target, now); err != nil {
			return err
		}
		// Review L2: per-pair cooldown after a decline; cap on a target's waiting proposals.
		if last, err := q.LastDaoLuDecline(ctx, db.LastDaoLuDeclineParams{ProposerCharacterID: me.ID, TargetCharacterID: target}); err != nil {
			return err
		} else if last.Unix() > 0 && now.Sub(last) < s.Cfg.DeclineCooldown() {
			return ErrDeclinedRecently
		}
		if n, err := q.CountDaoLuIncomingOpen(ctx, db.CountDaoLuIncomingOpenParams{CharacterID: target, Now: now}); err != nil {
			return err
		} else if n >= s.Cfg.Bond.MaxIncomingPending {
			return &GateError{Side: "target", Blockers: []string{"inbox_full"}}
		}
		if n, err := q.CountDaoLuOpenOutgoing(ctx, db.CountDaoLuOpenOutgoingParams{CharacterID: me.ID, Now: now}); err != nil {
			return err
		} else if n > 0 {
			return ErrOpenProposal
		}
		if n, err := q.CountDaoLuProposalsDay(ctx, db.CountDaoLuProposalsDayParams{CharacterID: me.ID, GameDay: gameday.PG(now)}); err != nil {
			return err
		} else if n >= s.Cfg.Bond.DailyProposals {
			return ErrDailyLimit
		}
		pr, err := q.InsertDaoLuProposal(ctx, db.InsertDaoLuProposalParams{ProposerCharacterID: me.ID, TargetCharacterID: target, NetFlag: shared,
			IdempotencyKey: idemKey, GameDay: gameday.PG(now), CreatedAt: now, ExpiresAt: now.Add(s.Cfg.ProposalExpiry())})
		if err != nil {
			return err
		}
		if err := q.InsertDaoLuEvent(ctx, db.InsertDaoLuEventParams{ProposalID: uuidPG(pr.ID), Kind: "proposed", ActorCharacterID: uuidPG(me.ID),
			CreatedAt: now}); err != nil {
			return err
		}
		if shared {
			return q.InsertDaoLuEvent(ctx, db.InsertDaoLuEventParams{ProposalID: uuidPG(pr.ID), Kind: "net_flag", ActorCharacterID: uuidPG(me.ID),
				Reason: "shared_network", Note: "propose", CreatedAt: now})
		}
		return nil
	})
	if err != nil {
		return View{}, err
	}
	if shared {
		s.flagLog(ctx, "dao lu proposal between accounts on the same network (admin review)", me.ID, target)
	}
	return s.view(ctx, db.New(s.Pool), userID, me.ID)
}

// gates checks both sides; the first blocked side is returned as a *GateError.
func (s *Service) gates(ctx context.Context, q *db.Queries, self, target uuid.UUID, now time.Time) error {
	for _, side := range []struct {
		name string
		id   uuid.UUID
	}{{"self", self}, {"target", target}} {
		_, p, err := s.party(ctx, q, side.id)
		if err != nil {
			return err
		}
		if b := s.Cfg.Blockers(p, now); len(b) > 0 {
			return &GateError{Side: side.name, Blockers: b}
		}
	}
	return nil
}

// Accept answers an incoming proposal; ack = the target confirmed the "vĩnh viễn" warning (the second confirmation).
// Lock order: proposal (FOR UPDATE) → bond insert → members in character_id order (the partial unique index makes a
// concurrent second bond for either character fail with ErrAlreadyBonded). Idempotent by key.
func (s *Service) Accept(ctx context.Context, userID, proposalID uuid.UUID, ack bool, idemKey string) (View, error) {
	if !ack {
		return View{}, ErrNotAcknowledged
	}
	var me db.Character
	var flagged bool
	var pair [2]uuid.UUID
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		var err error
		if me, err = s.me(ctx, q, userID); err != nil {
			return err
		}
		pr, err := q.GetDaoLuProposal(ctx, proposalID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrProposalGone
		} else if err != nil {
			return err
		}
		if pr.TargetCharacterID != me.ID {
			return ErrNotYourProposal
		}
		// Lock all pending proposals of both characters in id order (this one included), then re-read it.
		if _, err := q.LockDaoLuPendingFor(ctx, []uuid.UUID{pr.ProposerCharacterID, me.ID}); err != nil {
			return err
		}
		if pr, err = q.GetDaoLuProposalForUpdate(ctx, proposalID); err != nil {
			return err
		}
		now := s.Now().UTC()
		if pr.Status != "pending" {
			if pr.Status == "accepted" && pr.RespondKey.String == idemKey {
				return nil // replay
			}
			return ErrProposalGone
		}
		if !now.Before(pr.ExpiresAt) {
			return ErrProposalGone
		}
		if err := s.gates(ctx, q, me.ID, pr.ProposerCharacterID, now); err != nil {
			return err
		}
		proposer, err := q.GetDaoLuParty(ctx, pr.ProposerCharacterID)
		if err != nil {
			return err
		}
		shared, err := netsig.SharedRecentlyQ(ctx, q, proposer.UserID, userID, now)
		if err != nil {
			return fmt.Errorf("net signal: %w", err)
		}
		flagged = shared || pr.NetFlag
		if _, err := q.RespondDaoLuProposal(ctx, db.RespondDaoLuProposalParams{ID: pr.ID, Status: "accepted", RespondedAt: &now,
			RespondKey: pgtype.Text{String: idemKey, Valid: true}}); err != nil {
			return err
		}
		bond, err := q.InsertDaoLuBond(ctx, db.InsertDaoLuBondParams{ProposalID: pr.ID, CharacterA: pr.ProposerCharacterID, CharacterB: me.ID,
			NetFlag: flagged, FormedAt: now})
		if err != nil {
			return err
		}
		ids := []uuid.UUID{pr.ProposerCharacterID, me.ID}
		slices.SortFunc(ids, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
		pair = [2]uuid.UUID{ids[0], ids[1]}
		for _, id := range ids {
			if err := q.InsertDaoLuMember(ctx, db.InsertDaoLuMemberParams{BondID: bond.ID, CharacterID: id}); err != nil {
				var pg *pgconn.PgError
				if errors.As(err, &pg) && pg.Code == "23505" && pg.ConstraintName == errUniqueActiveBond {
					return ErrAlreadyBonded
				}
				return err
			}
		}
		if err := q.SupersedeDaoLuProposals(ctx, db.SupersedeDaoLuProposalsParams{Now: &now, ExceptID: pr.ID, Ids: ids}); err != nil {
			return err
		}
		if err := q.InsertDaoLuEvent(ctx, db.InsertDaoLuEventParams{BondID: uuidPG(bond.ID), ProposalID: uuidPG(pr.ID), Kind: "bonded",
			ActorCharacterID: uuidPG(me.ID), CreatedAt: now}); err != nil {
			return err
		}
		if shared && !pr.NetFlag {
			return q.InsertDaoLuEvent(ctx, db.InsertDaoLuEventParams{BondID: uuidPG(bond.ID), ProposalID: uuidPG(pr.ID), Kind: "net_flag",
				ActorCharacterID: uuidPG(me.ID), Reason: "shared_network", Note: "accept", CreatedAt: now})
		}
		return nil
	})
	if err != nil {
		return View{}, err
	}
	if flagged {
		s.flagLog(ctx, "dao lu bond between accounts on the same network (admin review)", pair[0], pair[1])
	}
	return s.view(ctx, db.New(s.Pool), userID, me.ID)
}

// Decline (target) or Cancel (proposer) closes a pending proposal. Idempotent by key.
func (s *Service) Decline(ctx context.Context, userID, proposalID uuid.UUID, idemKey string) (View, error) {
	return s.close(ctx, userID, proposalID, idemKey, "declined")
}

// Cancel withdraws the viewer's own pending proposal.
func (s *Service) Cancel(ctx context.Context, userID, proposalID uuid.UUID, idemKey string) (View, error) {
	return s.close(ctx, userID, proposalID, idemKey, "cancelled")
}

func (s *Service) close(ctx context.Context, userID, proposalID uuid.UUID, idemKey, status string) (View, error) {
	var me db.Character
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		var err error
		if me, err = s.me(ctx, q, userID); err != nil {
			return err
		}
		pr, err := q.GetDaoLuProposalForUpdate(ctx, proposalID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrProposalGone
		} else if err != nil {
			return err
		}
		mine := pr.TargetCharacterID
		if status == "cancelled" {
			mine = pr.ProposerCharacterID
		}
		if mine != me.ID {
			return ErrNotYourProposal
		}
		now := s.Now().UTC()
		if pr.Status != "pending" {
			if pr.Status == status && pr.RespondKey.String == idemKey {
				return nil
			}
			return ErrProposalGone
		}
		if !now.Before(pr.ExpiresAt) {
			return ErrProposalGone
		}
		if _, err := q.RespondDaoLuProposal(ctx, db.RespondDaoLuProposalParams{ID: pr.ID, Status: status, RespondedAt: &now,
			RespondKey: pgtype.Text{String: idemKey, Valid: true}}); err != nil {
			return err
		}
		return q.InsertDaoLuEvent(ctx, db.InsertDaoLuEventParams{ProposalID: uuidPG(pr.ID), Kind: status, ActorCharacterID: uuidPG(me.ID), CreatedAt: now})
	})
	if err != nil {
		return View{}, err
	}
	return s.view(ctx, db.New(s.Pool), userID, me.ID)
}

// ───── endings ─────

// EndMeta describes who/why for the audit trail.
type EndMeta struct {
	AdminUserID uuid.UUID // admin endings
	ReportID    uuid.UUID
	Note        string
}

// EndLocked ends an active bond inside the caller's transaction (lock: the bond row). ErrBondEnded if already ended.
func EndLocked(ctx context.Context, q *db.Queries, bondID uuid.UUID, reason string, meta EndMeta, now time.Time) error {
	switch reason {
	case ReasonLuanHoi, ReasonPhongThan, ReasonAdmin, ReasonAbsence:
	default:
		return ErrBadReason
	}
	b, err := q.GetDaoLuBondForUpdate(ctx, bondID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoBond
	} else if err != nil {
		return err
	}
	if b.EndedAt != nil {
		return ErrBondEnded
	}
	if _, err := q.EndDaoLuBond(ctx, db.EndDaoLuBondParams{ID: b.ID, EndedAt: &now, EndReason: pgtype.Text{String: reason, Valid: true}}); err != nil {
		return err
	}
	if err := q.EndDaoLuMembers(ctx, db.EndDaoLuMembersParams{BondID: b.ID, EndedAt: &now}); err != nil {
		return err
	}
	ev := db.InsertDaoLuEventParams{BondID: uuidPG(b.ID), ProposalID: uuidPG(b.ProposalID), Kind: "ended", Reason: reason, Note: meta.Note, CreatedAt: now}
	if meta.AdminUserID != uuid.Nil {
		ev.AdminUserID = uuidPG(meta.AdminUserID)
	}
	if meta.ReportID != uuid.Nil {
		ev.ReportID = uuidPG(meta.ReportID)
	}
	return q.InsertDaoLuEvent(ctx, ev)
}

// EndForCharacterLocked is the lượng kiếp hook: luân hồi (G4, Phase 6) and Phong Thần (Phase 8) call it inside their
// own transaction when a character "dies" in the story; it ends that character's active bond (if any) so both may bond
// again after the cooldown. Returns whether a bond was ended.
func EndForCharacterLocked(ctx context.Context, q *db.Queries, charID uuid.UUID, reason string, now time.Time) (bool, error) {
	if reason != ReasonLuanHoi && reason != ReasonPhongThan {
		return false, ErrBadReason
	}
	b, err := q.GetActiveDaoLuBond(ctx, charID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if err := EndLocked(ctx, q, b.ID, reason, EndMeta{Note: "character " + charID.String()}, now); err != nil {
		return false, err
	}
	return true, nil
}

// AdminEnd ends a bond after a report (harassment…). The report must be by or about one of the partners; the note is
// required. Writes dao_lu_events + admin_audit_logs in the same transaction.
func (s *Service) AdminEnd(ctx context.Context, adminUserID, bondID, reportID uuid.UUID, note string) error {
	note = strings.TrimSpace(note)
	if note == "" || len([]rune(note)) > 1000 {
		return ErrNoteRequired
	}
	return postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		now := s.Now().UTC()
		b, err := q.GetDaoLuBondForUpdate(ctx, bondID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoBond
		} else if err != nil {
			return err
		}
		rep, err := q.GetReportParties(ctx, reportID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReportMismatch
		} else if err != nil {
			return err
		}
		pa, err := q.GetDaoLuParty(ctx, b.CharacterA)
		if err != nil {
			return err
		}
		pb, err := q.GetDaoLuParty(ctx, b.CharacterB)
		if err != nil {
			return err
		}
		// Review L3: a report that is not dismissed AND either (a) filed by one partner against the other, or (b) about a
		// partner and already actioned by a moderator. A report by an outsider still pending review is not enough.
		between := (rep.ReporterUserID == pa.UserID && rep.TargetCharacterID == pb.ID) ||
			(rep.ReporterUserID == pb.UserID && rep.TargetCharacterID == pa.ID)
		aboutActioned := rep.Status == "actioned" && (rep.TargetCharacterID == pa.ID || rep.TargetCharacterID == pb.ID)
		if rep.Status == "dismissed" || (!between && !aboutActioned) {
			return ErrReportMismatch
		}
		if err := EndLocked(ctx, q, bondID, ReasonAdmin, EndMeta{AdminUserID: adminUserID, ReportID: reportID, Note: note}, now); err != nil {
			return err
		}
		before, _ := json.Marshal(map[string]any{"bondId": b.ID, "characterA": b.CharacterA, "characterB": b.CharacterB, "formedAt": b.FormedAt})
		after, _ := json.Marshal(map[string]any{"endedAt": now, "endReason": ReasonAdmin, "reportId": reportID})
		return q.InsertAdminAudit(ctx, db.InsertAdminAuditParams{AdminUserID: adminUserID, Action: "dao_lu.terminate", TargetType: "dao_lu_bond",
			TargetID: b.ID.String(), Before: before, After: after, Reason: pgtype.Text{String: note, Valid: true}})
	})
}

// EndAbsent ends up to limit bonds where a partner shows no play for absence_days (worker; one short tx per bond).
func (s *Service) EndAbsent(ctx context.Context, limit int) (int, error) {
	now := s.Now().UTC()
	cutoff := now.Add(-s.Cfg.Absence())
	ids, err := db.New(s.Pool).ListAbsentDaoLuBonds(ctx, db.ListAbsentDaoLuBondsParams{Cutoff: cutoff, Lim: int32(limit)})
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
			q := db.New(tx)
			b, err := q.GetDaoLuBondForUpdate(ctx, id)
			if err != nil {
				return err
			}
			if b.EndedAt != nil {
				return nil
			}
			absent := ""
			for _, c := range []uuid.UUID{b.CharacterA, b.CharacterB} {
				seen, err := q.DaoLuLastSeen(ctx, c)
				if err != nil {
					return err
				}
				if seen.Before(cutoff) {
					absent = c.String()
				}
			}
			if absent == "" {
				return nil // came back meanwhile
			}
			n++
			return EndLocked(ctx, q, b.ID, ReasonAbsence, EndMeta{Note: "absent " + absent}, now)
		})
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// Flag is one admin-review entry: a proposal/bond between accounts that shared a network.
type Flag struct {
	ProposalID   string    `json:"proposalId"`
	BondID       string    `json:"bondId,omitempty"`
	ProposerID   string    `json:"proposerId"`
	ProposerName string    `json:"proposerName"`
	TargetID     string    `json:"targetId"`
	TargetName   string    `json:"targetName"`
	Status       string    `json:"status"`
	Stage        string    `json:"stage"` // propose | accept
	FlaggedAt    time.Time `json:"flaggedAt"`
}

// Flags lists shared-network flags of the last `days` days (≤ 200, newest first). The read itself is audited
// (admin_audit_logs dao_lu.flags.read, review L7). Flags older than netsig.RetentionDays are purged (PurgeNetFlags).
func (s *Service) Flags(ctx context.Context, adminUserID uuid.UUID, days int) ([]Flag, error) {
	q := db.New(s.Pool)
	params, _ := json.Marshal(map[string]any{"days": days})
	if err := q.InsertAdminAudit(ctx, db.InsertAdminAuditParams{AdminUserID: adminUserID, Action: "dao_lu.flags.read", TargetType: "admin_read",
		TargetID: "dao_lu_flags", After: params}); err != nil {
		return nil, err
	}
	rows, err := q.ListDaoLuFlags(ctx, db.ListDaoLuFlagsParams{Since: s.Now().UTC().AddDate(0, 0, -max(1, days)), Lim: 200})
	if err != nil {
		return nil, err
	}
	out := make([]Flag, 0, len(rows))
	for _, r := range rows {
		f := Flag{ProposerID: r.ProposerCharacterID.String(), ProposerName: r.ProposerName, TargetID: r.TargetCharacterID.String(),
			TargetName: r.TargetName, Status: r.Status, Stage: r.Note, FlaggedAt: r.CreatedAt}
		if r.ProposalID.Valid {
			f.ProposalID = uuid.UUID(r.ProposalID.Bytes).String()
		}
		if r.BondID.Valid {
			f.BondID = uuid.UUID(r.BondID.Bytes).String()
		}
		out = append(out, f)
	}
	return out, nil
}

// PurgeNetFlags enforces the netsig retention on đạo lữ flags (review M2): clears net_flag on proposals/bonds and
// deletes kind='net_flag' events older than netsig.RetentionDays, each in batches of `batch` (worker, bounded).
func (s *Service) PurgeNetFlags(ctx context.Context, batch int) (int64, error) {
	cutoff := s.Now().UTC().Add(-netsig.Retention)
	q := db.New(s.Pool)
	var total int64
	n, err := q.DeleteOldDaoLuNetFlagEvents(ctx, db.DeleteOldDaoLuNetFlagEventsParams{Cutoff: cutoff, Lim: int32(batch)})
	total += n
	if err != nil {
		return total, err
	}
	n, err = q.ClearOldDaoLuProposalFlags(ctx, db.ClearOldDaoLuProposalFlagsParams{Cutoff: cutoff, Lim: int32(batch)})
	total += n
	if err != nil {
		return total, err
	}
	n, err = q.ClearOldDaoLuBondFlags(ctx, db.ClearOldDaoLuBondFlagsParams{Cutoff: cutoff, Lim: int32(batch)})
	return total + n, err
}
