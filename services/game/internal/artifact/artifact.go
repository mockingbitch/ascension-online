// Package artifact is the pháp bảo slot (Phase 3.4): a character equips ONE owned artifact (items.yaml category
// artifact); its tribulation_bonus_bp fills the "items" part of the tribulation odds (decision F, capped by realms.yaml
// tribulation.item_bonus_max_bp). Artifacts drop only from bí cảnh, are never sold and never tradable (loaders).
//
// "One equipped" rather than "best held counts": the choice is visible and deliberate (Túi Đồ), the odds breakdown
// names the artifact, and future artifacts with other effects can share the slot. An equipped artifact that is no
// longer owned (admin removal) simply counts 0.
package artifact

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/db"
	"tiendao/game/internal/item"
	"tiendao/game/internal/platform/postgres"
)

var (
	ErrNoCharacter = errors.New("artifact: character not found")
	ErrNotArtifact = errors.New("artifact: item is not a pháp bảo")
	ErrNotOwned    = errors.New("artifact: pháp bảo not owned")
)

// Equipped is the character's slot as the API shows it.
type Equipped struct {
	ItemID             string     `json:"itemId"` // "" = none
	TribulationBonusBP int64      `json:"tribulationBonusBp"`
	EquippedAt         *time.Time `json:"equippedAt"`
}

// Service owns the equip transaction.
type Service struct {
	Pool  *pgxpool.Pool
	Items *item.Catalogue
	Now   func() time.Time
}

// BonusLocked returns the equipped artifact and its bonus, 0 when none or no longer owned. Read inside the caller's
// transaction (the breakthrough holds the progression lock; the inventory row is read, not locked: artifacts are never
// consumed or traded, so ownership cannot drop concurrently).
func BonusLocked(ctx context.Context, q *db.Queries, items *item.Catalogue, charID uuid.UUID) (string, int64, error) {
	if items == nil {
		return "", 0, nil
	}
	row, err := q.GetEquippedArtifact(ctx, charID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, nil
	} else if err != nil {
		return "", 0, fmt.Errorf("equipped artifact: %w", err)
	}
	d, ok := items.Get(row.ItemID)
	if !ok || d.Category != item.CategoryArtifact {
		return "", 0, nil
	}
	inv, err := q.GetInventoryItems(ctx, db.GetInventoryItemsParams{CharacterID: charID, ItemIds: []string{row.ItemID}})
	if err != nil {
		return "", 0, err
	}
	if len(inv) == 0 || inv[0].Quantity < 1 {
		return "", 0, nil
	}
	return d.ID, d.TribulationBonusBP, nil
}

// Get returns the character's slot.
func (s *Service) Get(ctx context.Context, userID uuid.UUID) (Equipped, error) {
	q := db.New(s.Pool)
	ch, err := q.GetCharacterByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Equipped{}, ErrNoCharacter
	} else if err != nil {
		return Equipped{}, err
	}
	return s.view(ctx, q, ch.ID)
}

func (s *Service) view(ctx context.Context, q *db.Queries, charID uuid.UUID) (Equipped, error) {
	row, err := q.GetEquippedArtifact(ctx, charID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Equipped{}, nil
	} else if err != nil {
		return Equipped{}, err
	}
	id, bp, err := BonusLocked(ctx, q, s.Items, charID)
	if err != nil {
		return Equipped{}, err
	}
	at := row.EquippedAt
	return Equipped{ItemID: id, TribulationBonusBP: bp, EquippedAt: &at}, nil
}

// Equip sets the slot to itemID ("" = unequip). Naturally idempotent: the same request leaves the same state, so the
// Idempotency-Key is accepted but not stored. The character row is the serializer (lock order: this touches only
// character_artifacts and reads inventory).
func (s *Service) Equip(ctx context.Context, userID uuid.UUID, itemID string) (Equipped, error) {
	var out Equipped
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		ch, err := q.GetCharacterByUser(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoCharacter
		} else if err != nil {
			return err
		}
		if itemID == "" {
			if err := q.UnequipArtifact(ctx, ch.ID); err != nil {
				return err
			}
			out = Equipped{}
			return nil
		}
		d, ok := s.Items.Get(itemID)
		if !ok || d.Category != item.CategoryArtifact {
			return ErrNotArtifact
		}
		inv, err := q.GetInventoryItems(ctx, db.GetInventoryItemsParams{CharacterID: ch.ID, ItemIds: []string{itemID}})
		if err != nil {
			return err
		}
		if len(inv) == 0 || inv[0].Quantity < 1 {
			return ErrNotOwned
		}
		if _, err := q.EquipArtifact(ctx, db.EquipArtifactParams{CharacterID: ch.ID, ItemID: itemID, EquippedAt: s.Now().UTC()}); err != nil {
			return err
		}
		out, err = s.view(ctx, q, ch.ID)
		return err
	})
	return out, err
}
