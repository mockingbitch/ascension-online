// Package shop is the spirit-stone shop (decision D7, data-model §8): content/catalog/shop.yaml listings, the pure
// purchase rules and the buy transaction. It is the first spirit-stone SINK; it may sell speed items, breakthrough
// REQUIREMENT items and non-progress cosmetics, never items that raise breakthrough/tribulation odds (CrossCheck).
package shop

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"

	"gopkg.in/yaml.v3"

	"tiendao/game/internal/item"
)

// Currency is the only currency the shop takes (S5: prices come from the catalogue only).
const Currency = "spirit_stone"

const (
	maxPrice         = 1_000_000_000 // typo guard
	maxLotSize       = 1000
	maxDailyLimit    = 1000
	maxLotsPerOrder  = 100
	maxTierGuardHigh = 12
)

// Listing is one shop entry: `lot_size` items of `item_id` for `price` stones per lot.
type Listing struct {
	ID             string `yaml:"id" json:"id"`
	ItemID         string `yaml:"item_id" json:"itemId"`
	LotSize        int64  `yaml:"lot_size" json:"lotSize"`
	Price          int64  `yaml:"price" json:"price"`
	DailyLimitLots int32  `yaml:"daily_limit_lots,omitempty" json:"dailyLimitLots,omitempty"` // 0 = unlimited
	// MinTier: Tiên Duyên level needed to buy. Allowed on cosmetic listings only (CrossCheck): Tiên Duyên never
	// gates progress (CLAUDE.md non-negotiable 3).
	MinTier int `yaml:"min_tier,omitempty" json:"minTier,omitempty"`
}

// Settings are shop-wide rules.
type Settings struct {
	Currency        string `yaml:"currency" json:"currency"`
	MaxLotsPerOrder int32  `yaml:"max_lots_per_order" json:"maxLotsPerOrder"`
}

// Catalogue is immutable after load.
type Catalogue struct {
	Settings Settings
	listings []Listing
	byID     map[string]*Listing
}

type file struct {
	Version  int       `yaml:"version"`
	Settings Settings  `yaml:"settings"`
	Listings []Listing `yaml:"listings"`
}

var slugRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// LoadFile parses and validates shop.yaml. Item references and the odds rule are checked by CrossCheck
// (content.LoadAll).
func LoadFile(path string) (*Catalogue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load parses and validates shop YAML.
func Load(r io.Reader) (*Catalogue, error) {
	var raw file
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("shop: decode: %w", err)
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if raw.Version != 1 {
		fail("unsupported version %d", raw.Version)
	}
	if raw.Settings.Currency != Currency {
		fail("settings.currency must be %s", Currency)
	}
	if raw.Settings.MaxLotsPerOrder < 1 || raw.Settings.MaxLotsPerOrder > maxLotsPerOrder {
		fail("settings.max_lots_per_order must be within 1..%d", maxLotsPerOrder)
	}
	if len(raw.Listings) == 0 {
		fail("no listings")
	}
	c := &Catalogue{Settings: raw.Settings, listings: raw.Listings, byID: map[string]*Listing{}}
	for i := range c.listings {
		l := &c.listings[i]
		if !slugRe.MatchString(l.ID) {
			fail("listing[%d]: invalid id %q", i, l.ID)
		}
		if _, dup := c.byID[l.ID]; dup {
			fail("listing %q: duplicate id", l.ID)
		}
		if !slugRe.MatchString(l.ItemID) {
			fail("listing %q: invalid item_id %q", l.ID, l.ItemID)
		}
		if l.LotSize < 1 || l.LotSize > maxLotSize {
			fail("listing %q: lot_size must be within 1..%d", l.ID, maxLotSize)
		}
		if l.Price < 1 || l.Price > maxPrice {
			fail("listing %q: price must be within 1..%d", l.ID, maxPrice)
		}
		if l.DailyLimitLots < 0 || l.DailyLimitLots > maxDailyLimit {
			fail("listing %q: daily_limit_lots must be absent or within 1..%d", l.ID, maxDailyLimit)
		}
		if l.MinTier < 0 || l.MinTier > maxTierGuardHigh {
			fail("listing %q: min_tier must be absent or within 1..%d", l.ID, maxTierGuardHigh)
		}
		c.byID[l.ID] = l
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("shop: invalid catalogue: %w", errors.Join(errs...))
	}
	return c, nil
}

// All returns listings in file order. Do not mutate.
func (c *Catalogue) All() []Listing { return c.listings }

// Get returns a listing by ID.
func (c *Catalogue) Get(id string) (*Listing, bool) { l, ok := c.byID[id]; return l, ok }

// shopSafeEffects are the only item effects a listing may carry (data-model §8.4): convenience (offline window) and
// capped speed points. Odds effects are never sellable.
var shopSafeEffects = []string{item.EffectOfflineCapBonus, item.EffectInstantPoints, item.EffectMaxUsePerDay}

// CrossCheck validates listings against items.yaml (data-model §8.4 S1, decision G):
//   - the item exists, is not an artifact and raises neither breakthrough nor tribulation odds;
//   - every effect key is shop-safe; items with any effect need a daily_limit_lots;
//   - min_tier only on cosmetic listings; maxTier = highest Tiên Duyên level;
//   - unique_per_character items sell one at a time.
//
// The non-progress sink of decision G is no longer a shop listing: cosmetics.yaml (decisions 2026-11-07 "Linh thạch
// sinks"); content.LoadAll refuses cosmetic listings here so cosmetics keep one id space.
func (c *Catalogue) CrossCheck(items *item.Catalogue, maxTier int) error {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	for _, l := range c.listings {
		d, ok := items.Get(l.ItemID)
		if !ok {
			fail("shop listing %q sells unknown item %q", l.ID, l.ItemID)
			continue
		}
		if d.Category == "artifact" || d.RaisesOdds() {
			fail("shop listing %q sells %q, which raises breakthrough/tribulation odds (never sellable)", l.ID, d.ID)
		}
		keys := d.EffectKeys()
		for _, k := range keys {
			if !slices.Contains(shopSafeEffects, k) {
				fail("shop listing %q: item effect %q is not shop-safe", l.ID, k)
			}
		}
		if len(keys) > 0 && l.DailyLimitLots == 0 {
			fail("shop listing %q: items with effects need daily_limit_lots", l.ID)
		}
		if l.MinTier > 0 && d.Category != "cosmetic" {
			fail("shop listing %q: min_tier is only allowed on cosmetic listings", l.ID)
		}
		if l.MinTier > maxTier {
			fail("shop listing %q: min_tier %d above the last Tiên Duyên level %d", l.ID, l.MinTier, maxTier)
		}
		if d.UniquePerCharacter && l.LotSize != 1 {
			fail("shop listing %q: unique_per_character items need lot_size 1", l.ID)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("shop: cross-reference errors: %w", errors.Join(errs...))
	}
	return nil
}

// ───── pure purchase rules ─────

// Domain errors (mapped to HTTP in httpapi).
var (
	ErrUnknownListing = errors.New("shop: unknown listing")
	ErrBadLots        = errors.New("shop: lots out of range")
	ErrDailyLimit     = errors.New("shop: daily purchase limit reached")
	ErrTierLocked     = errors.New("shop: Tiên Duyên level too low")
	ErrAlreadyOwned   = errors.New("shop: unique item already owned")
	ErrInsufficient   = errors.New("shop: not enough spirit stones")
	ErrNoCharacter    = errors.New("shop: character not found")
	ErrKeyReuse       = errors.New("shop: idempotency key reused with a different listing or lots")
)

// Locked reasons shown on a listing (empty = buyable).
const (
	LockedTier     = "tier"
	LockedOwned    = "owned"
	LockedSoldOut  = "sold_out"
	LockedNoStones = "stones"
)

// Order is the validated cost of buying `lots` of a listing.
type Order struct {
	Listing      *Listing
	Lots         int32
	ItemQuantity int64
	TotalPrice   int64
}

// Quote validates a purchase intent against the catalogue and today's state. Pure.
// boughtToday = lots of this listing bought this game day; owned = quantity of the item held; tier = Tiên Duyên level.
func (c *Catalogue) Quote(listingID string, lots int32, def *item.Def, boughtToday int32, owned int64, tier int, balance int64) (Order, error) {
	l, ok := c.Get(listingID)
	if !ok || def == nil || def.ID != l.ItemID {
		return Order{}, ErrUnknownListing
	}
	if lots < 1 || lots > c.Settings.MaxLotsPerOrder {
		return Order{}, ErrBadLots
	}
	if def.UniquePerCharacter && (lots != 1 || owned > 0) {
		if owned > 0 {
			return Order{}, ErrAlreadyOwned
		}
		return Order{}, ErrBadLots
	}
	if tier < l.MinTier {
		return Order{}, ErrTierLocked
	}
	if l.DailyLimitLots > 0 && boughtToday+lots > l.DailyLimitLots {
		return Order{}, ErrDailyLimit
	}
	o := Order{Listing: l, Lots: lots, ItemQuantity: l.LotSize * int64(lots), TotalPrice: l.Price * int64(lots)}
	if balance < o.TotalPrice {
		return o, ErrInsufficient
	}
	return o, nil
}

// Remaining is how many lots can still be bought today (-1 = unlimited).
func (l *Listing) Remaining(boughtToday int32) int32 {
	if l.DailyLimitLots == 0 {
		return -1
	}
	return max(0, l.DailyLimitLots-boughtToday)
}

// LockedReason is why a listing can't be bought right now ("" = buyable). Order: tier, owned, sold out, stones.
func (l *Listing) LockedReason(def *item.Def, boughtToday int32, owned int64, tier int, balance int64) string {
	switch {
	case tier < l.MinTier:
		return LockedTier
	case def != nil && def.UniquePerCharacter && owned > 0:
		return LockedOwned
	case l.Remaining(boughtToday) == 0:
		return LockedSoldOut
	case balance < l.Price:
		return LockedNoStones
	}
	return ""
}
