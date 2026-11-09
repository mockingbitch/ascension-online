package cosmetic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/appearance"
	"tiendao/game/internal/db"
	"tiendao/game/internal/economy"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/realm"
	"tiendao/game/internal/vip"
)

// Ledger reason of a purchase debit.
const reasonPurchase = "cosmetic_purchase"

// View is GET /cosmetics and the state returned by every mutation (the client replaces, never merges).
type View struct {
	Balance   int64      `json:"balance"`
	Tier      int        `json:"tier"`
	Season    string     `json:"season"`
	EraID     string     `json:"eraId"`
	Owned     []string   `json:"owned"`     // purchased + Tiên Duyên perks the level reaches
	Available []string   `json:"available"` // shop cosmetics inside their season/era window now
	Equipped  Appearance `json:"equipped"`
	// BodyRetryAt: when the next body change is allowed (null = now; 24 h cooldown, free re-pick after a path change).
	BodyRetryAt *time.Time `json:"bodyRetryAt"`
}

// BuyResult is POST /cosmetics/buy.
type BuyResult struct {
	CosmeticID string `json:"cosmeticId"`
	Price      int64  `json:"price"`
	Replayed   bool   `json:"replayed"`
	State      View   `json:"state"`
}

// EquipResult is POST /cosmetics/equip.
type EquipResult struct {
	Replayed bool `json:"replayed"`
	State    View `json:"state"`
}

// Service owns cosmetic purchases and the equipped look.
type Service struct {
	Pool *pgxpool.Pool
	Cat  *Catalogue
	// Looks + Realms: body / hair accessory / robe / pose (appearances.yaml). nil Looks → looks are not served and an
	// equip carrying one is refused.
	Looks  *appearance.Catalogue
	Realms *realm.Catalogue
	VIP    *vip.Table
	Eras   EraIndex
	Now    func() time.Time
}

func character(ctx context.Context, q *db.Queries, userID uuid.UUID) (db.Character, error) {
	ch, err := q.GetCharacterByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ch, ErrNoCharacter
	}
	return ch, err
}

// when reads the world era without a lock (hot path rule §2.3) and the game-day month.
func (s *Service) when(ctx context.Context, q *db.Queries, now time.Time) (When, string, error) {
	w := When{Month: gameday.Of(now).Month()}
	ws, err := q.GetWorldState(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return w, "", nil
	} else if err != nil {
		return w, "", err
	}
	if ws.EraID.Valid {
		if seq, ok := s.Eras.EraSeq(ws.EraID.String); ok {
			w.EraSeq = seq
		}
	}
	return w, ws.EraID.String, nil
}

func (s *Service) tier(ctx context.Context, q *db.Queries, userID uuid.UUID) (int, error) {
	points, err := q.GetVIPPoints(ctx, userID)
	if err != nil {
		return 0, err
	}
	return s.VIP.TierForPoints(points), nil
}

func (s *Service) purchased(ctx context.Context, q *db.Queries, charID uuid.UUID) (map[string]bool, error) {
	ids, err := q.ListOwnedCosmetics(ctx, charID)
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m, nil
}

// AppearanceFromRow converts the DB row (nil-safe: no row = nothing equipped).
func AppearanceFromRow(r *db.CharacterAppearance) Appearance {
	a := Appearance{CaveDecorationIDs: []string{}}
	if r == nil {
		return a
	}
	a.TitleID, a.AvatarFrameID, a.ChatAuraID = r.TitleID.String, r.AvatarFrameID.String, r.ChatAuraID.String
	if r.CaveDecorationIds != nil {
		a.CaveDecorationIDs = r.CaveDecorationIds
	}
	return a
}

// LoadAppearance reads a character's equipped look (no lock). Used by GET /character.
func LoadAppearance(ctx context.Context, q *db.Queries, charID uuid.UUID) (Appearance, error) {
	r, err := q.GetAppearance(ctx, charID)
	if errors.Is(err, pgx.ErrNoRows) {
		return AppearanceFromRow(nil), nil
	} else if err != nil {
		return Appearance{}, err
	}
	return AppearanceFromRow(&r), nil
}

// StoredLook is the look part of the DB row as chosen (empty fields = not chosen). nil-safe.
func StoredLook(r *db.CharacterAppearance) (appearance.Look, *time.Time) {
	if r == nil {
		return appearance.Look{}, nil
	}
	return appearance.Look{BodyID: r.BodyID.String, HairID: r.HairID.String, RobePaletteID: r.RobePaletteID.String,
		PoseID: r.PoseID.String}, r.BodyChangedAt
}

// LoadLook reads the stored look and the time of the last in-game body change (no lock).
func LoadLook(ctx context.Context, q *db.Queries, charID uuid.UUID) (appearance.Look, *time.Time, error) {
	r, err := q.GetAppearance(ctx, charID)
	if errors.Is(err, pgx.ErrNoRows) {
		return appearance.Look{}, nil, nil
	} else if err != nil {
		return appearance.Look{}, nil, err
	}
	l, at := StoredLook(&r)
	return l, at, nil
}

// pathOf is the character's current realm path (the look is shown and validated for it). Deliberately NOT locked:
// Equip holds the appearance row lock, and faction moves lock progression without touching appearance — taking the
// progression lock here would invert that order. A conversion committing in between only means the stored body is on
// the old path, which shows its pair and keeps the free re-pick (same outcome as the other order).
func (s *Service) pathOf(ctx context.Context, q *db.Queries, charID uuid.UUID) (realm.PathID, error) {
	p, err := q.GetProgression(ctx, charID)
	if err != nil {
		return "", fmt.Errorf("read progression: %w", err)
	}
	st, err := s.Realms.Stage(p.StageID)
	if err != nil {
		return "", err
	}
	return st.Path, nil
}

// View returns the caller's cosmetics state (read-only).
func (s *Service) View(ctx context.Context, userID uuid.UUID) (View, error) {
	q := db.New(s.Pool)
	ch, err := character(ctx, q, userID)
	if err != nil {
		return View{}, err
	}
	return s.view(ctx, q, userID, ch.ID, s.Now())
}

func (s *Service) view(ctx context.Context, q *db.Queries, userID, charID uuid.UUID, now time.Time) (View, error) {
	balance, err := q.GetCurrencyBalance(ctx, db.GetCurrencyBalanceParams{CharacterID: charID, Currency: Currency})
	if err != nil {
		return View{}, err
	}
	tier, err := s.tier(ctx, q, userID)
	if err != nil {
		return View{}, err
	}
	bought, err := s.purchased(ctx, q, charID)
	if err != nil {
		return View{}, err
	}
	w, era, err := s.when(ctx, q, now)
	if err != nil {
		return View{}, err
	}
	eq, err := LoadAppearance(ctx, q, charID)
	if err != nil {
		return View{}, err
	}
	v := View{Balance: balance, Tier: tier, Season: s.Cat.SeasonOf(w.Month), EraID: era, Owned: []string{}, Available: []string{},
		Equipped: s.Cat.VisibleLook(eq, tier)}
	if s.Looks != nil {
		stored, changedAt, err := LoadLook(ctx, q, charID)
		if err != nil {
			return View{}, err
		}
		path, err := s.pathOf(ctx, q, charID)
		if err != nil {
			return View{}, err
		}
		look := s.Looks.Resolve(stored, path)
		v.Equipped.Look = &look
		v.BodyRetryAt = s.Looks.BodyRetryAt(stored, changedAt, path, now)
	}
	for i := range s.Cat.All() {
		cm := &s.Cat.All()[i]
		if Owns(cm, bought, tier) {
			v.Owned = append(v.Owned, cm.ID)
		}
		if s.Cat.Available(cm, w, s.Eras) {
			v.Available = append(v.Available, cm.ID)
		}
	}
	return v, nil
}

// Buy purchases one shop cosmetic in ONE transaction: lock currency account → idempotency (character_cosmetics key)
// → quote (catalogue price, window, Tiên Duyên gate, owned once) → debit with ledger → ownership row (PK = owned at
// most once). Lock order (§2.3): currency_accounts only. No outbox: a cosmetic changes nothing but looks.
func (s *Service) Buy(ctx context.Context, userID uuid.UUID, cosmeticID, idemKey string) (BuyResult, error) {
	if _, ok := s.Cat.Get(cosmeticID); !ok {
		return BuyResult{}, ErrUnknown
	}
	var out BuyResult
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		ch, err := character(ctx, q, userID)
		if err != nil {
			return err
		}
		if err := q.EnsureCurrencyAccount(ctx, db.EnsureCurrencyAccountParams{CharacterID: ch.ID, Currency: Currency}); err != nil {
			return err
		}
		acc, err := q.GetCurrencyAccountForUpdate(ctx, db.GetCurrencyAccountForUpdateParams{CharacterID: ch.ID, Currency: Currency})
		if err != nil {
			return fmt.Errorf("lock currency: %w", err)
		}
		now := s.Now()
		if prev, err := q.GetCosmeticPurchaseByKey(ctx, db.GetCosmeticPurchaseByKeyParams{CharacterID: ch.ID, IdempotencyKey: idemKey}); err == nil {
			if prev.CosmeticID != cosmeticID {
				return ErrKeyReuse
			}
			out = BuyResult{CosmeticID: prev.CosmeticID, Price: prev.Price, Replayed: true}
			out.State, err = s.view(ctx, q, userID, ch.ID, now)
			return err
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		owned, err := q.CosmeticOwned(ctx, db.CosmeticOwnedParams{CharacterID: ch.ID, CosmeticID: cosmeticID})
		if err != nil {
			return err
		}
		tier, err := s.tier(ctx, q, userID)
		if err != nil {
			return err
		}
		w, _, err := s.when(ctx, q, now)
		if err != nil {
			return err
		}
		cm, err := s.Cat.Quote(cosmeticID, owned, tier, acc.Balance, w, s.Eras)
		if err != nil {
			return err
		}
		ref := economy.Ref{Reason: reasonPurchase, Type: "cosmetic", ID: cm.ID, IdemKey: "cosmetic:" + idemKey}
		if _, err := economy.Credit(ctx, q, ch.ID, Currency, -cm.Price, ref); errors.Is(err, economy.ErrInsufficient) {
			return ErrInsufficient
		} else if err != nil {
			return fmt.Errorf("debit: %w", err)
		}
		if err := q.InsertCosmeticOwnership(ctx, db.InsertCosmeticOwnershipParams{
			CharacterID: ch.ID, CosmeticID: cm.ID, Source: "purchase", Price: cm.Price, IdempotencyKey: idemKey, Now: now,
		}); err != nil {
			return fmt.Errorf("insert ownership: %w", err)
		}
		out = BuyResult{CosmeticID: cm.ID, Price: cm.Price}
		out.State, err = s.view(ctx, q, userID, ch.ID, now)
		return err
	})
	return out, err
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

// Equip sets the whole loadout (empty slot = unequip) in ONE transaction: ensure + lock the appearance row →
// idempotency (cosmetic_equips key; same key + same loadout = replay, other loadout = ErrKeyReuse) → validate
// ownership (purchases never disappear; Tiên Duyên perks by the current level) → update + log row.
func (s *Service) Equip(ctx context.Context, userID uuid.UUID, want Appearance, idemKey string) (EquipResult, error) {
	if want.CaveDecorationIDs == nil {
		want.CaveDecorationIDs = []string{}
	}
	if want.Look != nil && s.Looks != nil {
		// body + outfit + pose; accessory and robe come from the outfit (decision "Bộ trang phục vẽ sẵn")
		l := s.Looks.Normalize(*want.Look)
		want.Look = &l
	}
	var out EquipResult
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		ch, err := character(ctx, q, userID)
		if err != nil {
			return err
		}
		if err := q.EnsureAppearance(ctx, ch.ID); err != nil {
			return err
		}
		row, err := q.GetAppearanceForUpdate(ctx, ch.ID)
		if err != nil {
			return fmt.Errorf("lock appearance: %w", err)
		}
		now := s.Now()
		if prev, err := q.GetCosmeticEquipByKey(ctx, db.GetCosmeticEquipByKeyParams{CharacterID: ch.ID, IdempotencyKey: idemKey}); err == nil {
			got := Appearance{TitleID: prev.TitleID.String, AvatarFrameID: prev.AvatarFrameID.String, ChatAuraID: prev.ChatAuraID.String,
				CaveDecorationIDs: prev.CaveDecorationIds}
			if prev.LookSet {
				got.Look = &appearance.Look{BodyID: prev.BodyID.String, HairID: prev.HairID.String,
					RobePaletteID: prev.RobePaletteID.String, PoseID: prev.PoseID.String}
				if s.Looks != nil {
					*got.Look = s.Looks.Normalize(*got.Look) // the log stores the outfit as its parts
				}
			}
			if got.CaveDecorationIDs == nil {
				got.CaveDecorationIDs = []string{}
			}
			if !got.Equal(want) {
				return ErrKeyReuse
			}
			out.Replayed = true
			out.State, err = s.view(ctx, q, userID, ch.ID, now)
			return err
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		tier, err := s.tier(ctx, q, userID)
		if err != nil {
			return err
		}
		bought, err := s.purchased(ctx, q, ch.ID)
		if err != nil {
			return err
		}
		if err := s.Cat.Validate(want, func(cm *Cosmetic) bool { return Owns(cm, bought, tier) }); err != nil {
			return err
		}
		// Look (display only): valid for the current path, free/owned, body cooldown (24 h; free re-pick after a
		// path change). The stored body is kept when the requested one is already shown (Plan).
		var lookLog db.InsertCosmeticEquipParams
		if want.Look != nil {
			if s.Looks == nil {
				return fmt.Errorf("%w: looks are not available", ErrBadLoadout)
			}
			path, err := s.pathOf(ctx, q, ch.ID)
			if err != nil {
				return err
			}
			if err := s.Looks.Validate(*want.Look, path); err != nil {
				return err
			}
			stored, changedAt := StoredLook(&row)
			plan, err := s.Looks.Plan(stored, changedAt, *want.Look, path, now)
			if err != nil {
				return err
			}
			if plan.BodyChanged {
				changedAt = &now
			}
			if err := q.SetLook(ctx, db.SetLookParams{CharacterID: ch.ID, BodyID: text(plan.Store.BodyID), HairID: text(plan.Store.HairID),
				RobePaletteID: text(plan.Store.RobePaletteID), PoseID: text(plan.Store.PoseID), BodyChangedAt: changedAt, Now: now}); err != nil {
				return fmt.Errorf("set look: %w", err)
			}
			lookLog = db.InsertCosmeticEquipParams{LookSet: true, BodyID: text(want.Look.BodyID), HairID: text(want.Look.HairID),
				RobePaletteID: text(want.Look.RobePaletteID), PoseID: text(want.Look.PoseID)}
		}
		if err := q.SetAppearance(ctx, db.SetAppearanceParams{
			CharacterID: ch.ID, TitleID: text(want.TitleID), AvatarFrameID: text(want.AvatarFrameID), ChatAuraID: text(want.ChatAuraID),
			CaveDecorationIds: want.CaveDecorationIDs, Now: now,
		}); err != nil {
			return fmt.Errorf("set appearance: %w", err)
		}
		if err := q.InsertCosmeticEquip(ctx, db.InsertCosmeticEquipParams{
			CharacterID: ch.ID, TitleID: text(want.TitleID), AvatarFrameID: text(want.AvatarFrameID), ChatAuraID: text(want.ChatAuraID),
			CaveDecorationIds: want.CaveDecorationIDs, IdempotencyKey: idemKey, Now: now,
			LookSet: lookLog.LookSet, BodyID: lookLog.BodyID, HairID: lookLog.HairID, RobePaletteID: lookLog.RobePaletteID, PoseID: lookLog.PoseID,
		}); err != nil {
			return fmt.Errorf("equip log: %w", err)
		}
		out.State, err = s.view(ctx, q, userID, ch.ID, now)
		return err
	})
	return out, err
}
