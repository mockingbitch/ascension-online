package character

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/appearance"
	"tiendao/game/internal/cosmetic"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/db"
	"tiendao/game/internal/faction"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/leaderboard"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/realm"
	"tiendao/game/internal/vip"
	"tiendao/game/internal/world"
)

var (
	ErrAlreadyExists = errors.New("character: account already has a character")
	ErrNameTaken     = errors.New("character: name already taken")
	ErrNotFound      = cultivation.ErrNoCharacter
	// ErrNoFactions: onboarding with a faction but the faction service is not wired.
	ErrNoFactions = errors.New("character: factions unavailable")
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
	FailStreak          int32               `json:"failStreak"`
	CooldownUntil       *time.Time          `json:"cooldownUntil"`
	StageReachedAt      time.Time           `json:"stageReachedAt"`
	OfflineBonusSeconds int32               `json:"offlineBonusSeconds"` // pending Tụ Linh Phù bonus
	BonusUsedToday      bool                `json:"bonusUsedToday"`
	FactionID           *string             `json:"factionId"`          // null = no faction yet (onboarding sends them to the faction step)
	Appearance          cosmetic.Appearance `json:"appearance"`         // equipped cosmetics (ids only; GET /cosmetics/catalog resolves them)
	Replayed            bool                `json:"replayed,omitempty"` // onboarding create replayed by Idempotency-Key
}

// NameChecker is the moderation filter (moderation.Catalogue). It receives a format-normalised name and returns
// an error wrapping moderation.ErrNameNotAllowed when the name is blocked. Nil → no filtering.
type NameChecker interface {
	CheckName(name string) error
}

// Service creates and reads characters.
type Service struct {
	Pool        *pgxpool.Pool
	Cultivation *cultivation.Service
	Names       NameChecker
	Factions    *faction.Service // nil → onboarding with a faction is refused (ErrNoFactions)
	// Cosmetics + VIP hide Tiên Duyên perk cosmetics above the current level in the shown look (defence in depth; vip
	// strips them on a level drop). nil → shown as stored.
	Cosmetics *cosmetic.Catalogue
	VIP       *vip.Table
	// Looks: body / accessory / robe / pose (appearances.yaml, display only). nil → no look in the view and an
	// onboarding appearance is refused.
	Looks *appearance.Catalogue
}

// Onboarding is the faction part of the new-player intent POST /character {name, factionId, path} (data-model §9.6).
type Onboarding struct {
	IdempotencyKey string
	FactionID      string
	Path           realm.PathID // "" = the faction's first allowed path
	// Look picked in the onboarding step (optional; nil = the path default). Validated for the resolved path; the
	// pick does not start the body-change cooldown.
	Look *appearance.Look
}

// ErrNoLooks: an onboarding appearance was sent but the appearance catalogue is not wired.
var ErrNoLooks = errors.New("character: appearances unavailable")

// ValidName normalises the format, then applies the moderation filter.
func ValidName(raw string, names NameChecker) (string, error) {
	name, err := NormalizeName(raw)
	if err != nil {
		return "", err
	}
	if names != nil {
		if err := names.CheckName(name); err != nil {
			return "", err
		}
	}
	return name, nil
}

// Rename sets a character's name inside the caller's transaction (admin moderation). Returns the old and new name.
// Errors: ErrNotFound, ErrInvalidName, moderation name errors, ErrNameTaken.
func Rename(ctx context.Context, q *db.Queries, id uuid.UUID, raw string, names NameChecker) (oldName, newName string, err error) {
	newName, err = ValidName(raw, names)
	if err != nil {
		return "", "", err
	}
	ch, err := q.GetCharacterForUpdate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrNotFound
	} else if err != nil {
		return "", "", err
	}
	if ch.Name == newName {
		return ch.Name, newName, nil
	}
	// The old đạo hiệu leaves the public news history with the rename (moderation; GĐ-D review M3).
	if _, err := q.DeleteWorldAnnouncementsOfCharacter(ctx, pgtype.UUID{Bytes: id, Valid: true}); err != nil {
		return "", "", err
	}
	// A unique violation aborts the caller's transaction: it must roll back (the whole resolution fails).
	if err := q.RenameCharacter(ctx, db.RenameCharacterParams{ID: id, Name: newName}); err != nil {
		return "", "", mapUnique(err)
	}
	return ch.Name, newName, nil
}

// Create makes the account's (single) character at the first stage of the default path. If the world started
// more than late_join_grant.after_server_day days ago, the one-time late-join grant (50% of the server median) is
// stored as grant_reserve_milli — at most once per account (late_join_grants PK), released by the credit path.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, rawName string) (View, error) {
	return s.create(ctx, userID, rawName, nil)
}

// CreateOnboarded is the new-player intent (data-model §9.6): one transaction creates the character at the first
// stage of the path the faction gives (requested path if the faction allows it, else its first path) and joins the
// faction (character_faction + faction_history + outbox, the first join starts the switch cooldown, Phase 2.2).
// Idempotent: a retry with the same key, name and faction returns the current view with Replayed = true.
// Errors: those of Create, plus faction errors (ErrUnknownFaction, ErrFactionClosed, ErrPathNotAllowed,
// ErrIdempotencyReused) — on any error nothing is written.
func (s *Service) CreateOnboarded(ctx context.Context, userID uuid.UUID, rawName string, o Onboarding) (View, error) {
	if s.Factions == nil {
		return View{}, ErrNoFactions
	}
	return s.create(ctx, userID, rawName, &o)
}

func (s *Service) create(ctx context.Context, userID uuid.UUID, rawName string, o *Onboarding) (View, error) {
	name, err := ValidName(rawName, s.Names)
	if err != nil {
		return View{}, err
	}
	cat := s.Cultivation.Realms
	path := cat.DefaultPath()
	if o != nil {
		f, ok := s.Factions.Cat.Get(o.FactionID)
		if !ok {
			return View{}, faction.ErrUnknownFaction
		}
		if path, err = faction.ResolvePath(f, "", o.Path); err != nil {
			return View{}, err
		}
		if o.Look != nil {
			if s.Looks == nil {
				return View{}, ErrNoLooks
			}
			// body + outfit + pose; accessory and robe come from the outfit (decision "Bộ trang phục vẽ sẵn")
			l := s.Looks.Normalize(*o.Look)
			o.Look = &l
			if err := s.Looks.Validate(*o.Look, path); err != nil {
				return View{}, err
			}
		}
	}
	replayed := false
	err = postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		if old, err := q.GetCharacterByUser(ctx, userID); err == nil {
			if o == nil || old.Name != name {
				return ErrAlreadyExists
			}
			joined, err := faction.JoinedWithKey(ctx, q, old.ID, o.IdempotencyKey, o.FactionID)
			if err != nil {
				return err
			}
			if !joined {
				return ErrAlreadyExists
			}
			// A replay must carry the same look as the original intent (like Equip's key check).
			if o.Look != nil {
				stored, _, err := cosmetic.LoadLook(ctx, q, old.ID)
				if err != nil {
					return err
				}
				if s.Looks.Normalize(stored) != *o.Look {
					return faction.ErrIdempotencyReused
				}
			}
			replayed = true
			return nil
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
		first := cat.First(path)
		if first == nil {
			return fmt.Errorf("character: path %q has no stages", path)
		}
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
		if err := cultivation.EnqueueLeaderboard(ctx, q, ch.ID, leaderboard.Score(first, 0)); err != nil {
			return err
		}
		if o == nil {
			return nil
		}
		if o.Look != nil {
			if err := q.InsertOnboardingLook(ctx, db.InsertOnboardingLookParams{CharacterID: ch.ID, BodyID: text(o.Look.BodyID),
				HairID: text(o.Look.HairID), RobePaletteID: text(o.Look.RobePaletteID), PoseID: text(o.Look.PoseID), Now: now}); err != nil {
				return fmt.Errorf("onboarding look: %w", err)
			}
		}
		// Same tx: the faction checks (open in this era, path) run on the row just created; any refusal rolls the
		// character back too, so the player never ends up half-onboarded.
		_, err = s.Factions.JoinTx(ctx, q, userID, o.IdempotencyKey, o.FactionID, path)
		return err
	})
	if err != nil {
		return View{}, err
	}
	v, err := s.Get(ctx, userID)
	v.Replayed = replayed
	return v, err
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
	var factionID *string
	if cf, err := q.GetCharacterFaction(ctx, ch.ID); err == nil {
		factionID = &cf.FactionID
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return View{}, err
	}
	look, err := cosmetic.LoadAppearance(ctx, q, ch.ID)
	if err != nil {
		return View{}, err
	}
	if s.Cosmetics != nil && s.VIP != nil {
		points, err := q.GetVIPPoints(ctx, userID)
		if err != nil {
			return View{}, err
		}
		look = s.Cosmetics.VisibleLook(look, s.VIP.TierForPoints(points))
	}
	if s.Looks != nil {
		stored, _, err := cosmetic.LoadLook(ctx, q, ch.ID)
		if err != nil {
			return View{}, err
		}
		resolved := s.Looks.Resolve(stored, proj.Credit.After.Stage.Path)
		look.Look = &resolved
	}
	return View{
		FactionID: factionID, Appearance: look,
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

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
