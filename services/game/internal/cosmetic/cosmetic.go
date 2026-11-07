// Package cosmetic is the non-progress spirit-stone sink of decisions 2026-11-07 "Linh thạch sinks": titles, portrait
// frames, chat name effects and cave decorations (content/catalog/cosmetics.yaml), their ownership and the equipped
// look other players see.
//
// NO GAMEPLAY EFFECT: nothing in this package feeds cultivation, points, breakthrough/tribulation odds, ranking,
// faction merit or điểm duyên. The loader rejects effect-like fields (rejectEffects) so a designer cannot add one.
package cosmetic

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"tiendao/game/internal/item"
)

// Currency is the only currency cosmetics are priced in.
const Currency = "spirit_stone"

// Kind is the equip slot family; it is also the vip.yaml perk key ("title:vip_1").
type Kind string

const (
	KindTitle       Kind = "title"
	KindAvatarFrame Kind = "avatar_frame"
	KindChatAura    Kind = "chat_aura"
	KindCave        Kind = "cave_decoration"
)

// Kinds lists the cosmetic kinds (the vip perk keys this catalogue owns).
var Kinds = []Kind{KindTitle, KindAvatarFrame, KindChatAura, KindCave}

// Source says how a cosmetic is obtained.
type Source string

const (
	SourceShop      Source = "shop"       // bought with spirit stones (POST /cosmetics/buy)
	SourceTienDuyen Source = "tien_duyen" // granted while the Tiên Duyên level reaches the perk's tier (vip.yaml)
)

const (
	maxPrice     = 100_000_000 // typo guard
	maxCaveSlots = 12          // also a DB CHECK (character_appearance.cave_decoration_ids)
	maxTierGuard = 12
)

var (
	slugRe   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	rarities = map[string]bool{"pham": true, "linh": true, "huyen": true, "dia": true, "thien": true, "tien": true}
)

// Season is a game-day month window (UTC+7).
type Season struct {
	ID     string `yaml:"id" json:"id"`
	Name   string `yaml:"name" json:"name"`
	Months []int  `yaml:"months" json:"months"`
}

// Cosmetic is one entry. ID = "<kind>:<slug>".
type Cosmetic struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Kind        Kind   `json:"kind"`
	Series      string `json:"series"`
	Name        string `json:"name"`
	Rarity      string `json:"rarity"`
	Source      Source `json:"source"`
	Price       int64  `json:"price,omitempty"`   // shop only
	MinTier     int    `json:"minTier,omitempty"` // shop only; Tiên Duyên gate (cosmetic → allowed)
	PerkTier    int    `json:"perkTier,omitempty"`
	Description string `json:"description,omitempty"`
	Season      string `json:"season,omitempty"`
	FromEra     string `json:"fromEra,omitempty"`
	UntilEra    string `json:"untilEra,omitempty"`
}

// Series groups cosmetics of one kind (a price ladder or a decoration set).
type Series struct {
	ID       string   `json:"id"`
	Kind     Kind     `json:"kind"`
	Name     string   `json:"name"`
	Source   Source   `json:"source"`
	Season   string   `json:"season,omitempty"`
	FromEra  string   `json:"fromEra,omitempty"`
	UntilEra string   `json:"untilEra,omitempty"`
	Items    []string `json:"items"` // cosmetic ids in ladder order
}

// Settings are catalogue-wide rules.
type Settings struct {
	Currency  string `yaml:"currency" json:"currency"`
	CaveSlots int    `yaml:"cave_slots" json:"caveSlots"`
}

// Catalogue is immutable once content.LoadAll returned (BindPerks runs inside it, before sharing).
type Catalogue struct {
	Settings Settings
	Seasons  []Season
	series   []Series
	items    []Cosmetic
	byID     map[string]int
	slugs    map[string]bool
}

type fileItem struct {
	Slug        string `yaml:"slug"`
	Name        string `yaml:"name"`
	Rarity      string `yaml:"rarity"`
	Price       int64  `yaml:"price"`
	MinTier     int    `yaml:"min_tier"`
	Description string `yaml:"description"`
	// Extra catches every other key so an effect field fails with a clear message (rejectEffects) instead of a
	// generic decode error.
	Extra map[string]any `yaml:",inline"`
}

type fileSeries struct {
	ID       string         `yaml:"id"`
	Kind     Kind           `yaml:"kind"`
	Name     string         `yaml:"name"`
	Source   Source         `yaml:"source"`
	Season   string         `yaml:"season"`
	FromEra  string         `yaml:"from_era"`
	UntilEra string         `yaml:"until_era"`
	Items    []fileItem     `yaml:"items"`
	Extra    map[string]any `yaml:",inline"`
}

type file struct {
	Version  int          `yaml:"version"`
	Settings Settings     `yaml:"settings"`
	Seasons  []Season     `yaml:"seasons"`
	Series   []fileSeries `yaml:"series"`
}

// EffectLike reports whether a YAML key looks like a gameplay effect: any items.yaml effect key, any unit-suffixed
// number (`*_bp`, `*_milli`, `*_hours`, `*_seconds`) or a known modifier name. Cosmetics and faction works must not
// carry one (decisions 2026-11-07: sinks never speed progress).
func EffectLike(key string) bool {
	switch key {
	case item.EffectOfflineCapBonus, item.EffectInstantPoints, item.EffectMaxUsePerDay, item.EffectBreakthroughBP,
		item.EffectTribulationBP, "rate_bonus", "points", "merit", "rank_points", "vip_points", "cultivation":
		return true
	}
	for _, suf := range []string{"_bp", "_milli", "_hours", "_seconds", "_points"} {
		if strings.HasSuffix(key, suf) {
			return true
		}
	}
	return false
}

func rejectExtra(where string, extra map[string]any, fail func(string, ...any)) {
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if EffectLike(k) {
			fail("%s: field %q is a gameplay effect — cosmetics have no gameplay effect", where, k)
		} else {
			fail("%s: unknown field %q", where, k)
		}
	}
}

// LoadFile parses and validates cosmetics.yaml.
func LoadFile(path string) (*Catalogue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load parses and validates cosmetics YAML. Era names and vip perks are checked by CrossCheck/BindPerks.
func Load(r io.Reader) (*Catalogue, error) {
	var raw file
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("cosmetic: decode: %w", err)
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if raw.Version != 1 {
		fail("unsupported version %d", raw.Version)
	}
	if raw.Settings.Currency != Currency {
		fail("settings.currency must be %s", Currency)
	}
	if raw.Settings.CaveSlots < 1 || raw.Settings.CaveSlots > maxCaveSlots {
		fail("settings.cave_slots must be within 1..%d", maxCaveSlots)
	}
	c := &Catalogue{Settings: raw.Settings, Seasons: raw.Seasons, byID: map[string]int{}, slugs: map[string]bool{}}
	seasons := map[string]bool{}
	monthSeen := map[int]string{}
	for _, s := range raw.Seasons {
		if !slugRe.MatchString(s.ID) || seasons[s.ID] {
			fail("season %q: invalid or duplicate id", s.ID)
		}
		seasons[s.ID] = true
		if s.Name == "" || len(s.Months) == 0 {
			fail("season %q: name and months required", s.ID)
		}
		for _, m := range s.Months {
			if m < 1 || m > 12 {
				fail("season %q: month %d out of 1..12", s.ID, m)
			} else if prev, dup := monthSeen[m]; dup {
				fail("season %q: month %d already in season %q", s.ID, m, prev)
			}
			monthSeen[m] = s.ID
		}
	}
	seriesSeen := map[string]bool{}
	shopItems := 0
	for _, fs := range raw.Series {
		where := fmt.Sprintf("series %q", fs.ID)
		if !slugRe.MatchString(fs.ID) || seriesSeen[fs.ID] {
			fail("%s: invalid or duplicate id", where)
		}
		seriesSeen[fs.ID] = true
		rejectExtra(where, fs.Extra, fail)
		if !slices.Contains(Kinds, fs.Kind) {
			fail("%s: unknown kind %q", where, fs.Kind)
		}
		if fs.Name == "" {
			fail("%s: empty name", where)
		}
		if fs.Source == "" {
			fs.Source = SourceShop
		}
		if fs.Source != SourceShop && fs.Source != SourceTienDuyen {
			fail("%s: unknown source %q", where, fs.Source)
		}
		if fs.Season != "" && !seasons[fs.Season] {
			fail("%s: unknown season %q", where, fs.Season)
		}
		if fs.Source == SourceTienDuyen && (fs.Season != "" || fs.FromEra != "" || fs.UntilEra != "") {
			fail("%s: tien_duyen cosmetics have no availability window", where)
		}
		if fs.UntilEra != "" && fs.FromEra == "" {
			fail("%s: until_era needs from_era", where)
		}
		if len(fs.Items) == 0 {
			fail("%s: no items", where)
		}
		ser := Series{ID: fs.ID, Kind: fs.Kind, Name: fs.Name, Source: fs.Source, Season: fs.Season, FromEra: fs.FromEra, UntilEra: fs.UntilEra}
		var lastPrice int64
		for i, fi := range fs.Items {
			iw := fmt.Sprintf("%s item[%d] %q", where, i, fi.Slug)
			rejectExtra(iw, fi.Extra, fail)
			if !slugRe.MatchString(fi.Slug) {
				fail("%s: invalid slug", iw)
			}
			if c.slugs[fi.Slug] {
				fail("%s: duplicate slug (slugs are unique across cosmetics: art slot cosmetics/<slug>)", iw)
			}
			c.slugs[fi.Slug] = true
			if fi.Name == "" {
				fail("%s: empty name", iw)
			}
			if !rarities[fi.Rarity] {
				fail("%s: unknown rarity %q", iw, fi.Rarity)
			}
			switch fs.Source {
			case SourceShop:
				shopItems++
				if fi.Price < 1 || fi.Price > maxPrice {
					fail("%s: price must be within 1..%d", iw, maxPrice)
				}
				if fi.Price <= lastPrice {
					fail("%s: price %d must be above the previous item's %d (escalating ladder)", iw, fi.Price, lastPrice)
				}
				lastPrice = fi.Price
				if fi.MinTier < 0 || fi.MinTier > maxTierGuard {
					fail("%s: min_tier out of range", iw)
				}
			case SourceTienDuyen:
				if fi.Price != 0 || fi.MinTier != 0 {
					fail("%s: tien_duyen cosmetics have no price or min_tier (the vip.yaml perk tier grants them)", iw)
				}
			}
			cm := Cosmetic{ID: string(fs.Kind) + ":" + fi.Slug, Slug: fi.Slug, Kind: fs.Kind, Series: fs.ID, Name: fi.Name,
				Rarity: fi.Rarity, Source: fs.Source, Price: fi.Price, MinTier: fi.MinTier, Description: fi.Description,
				Season: fs.Season, FromEra: fs.FromEra, UntilEra: fs.UntilEra}
			c.byID[cm.ID] = len(c.items)
			c.items = append(c.items, cm)
			ser.Items = append(ser.Items, cm.ID)
		}
		c.series = append(c.series, ser)
	}
	if shopItems == 0 {
		fail("no shop cosmetics: decision G needs a non-progress spirit-stone sink")
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("cosmetic: invalid catalogue: %w", errors.Join(errs...))
	}
	return c, nil
}

// Get returns a cosmetic by full id ("title:so_tam").
func (c *Catalogue) Get(id string) (*Cosmetic, bool) {
	i, ok := c.byID[id]
	if !ok {
		return nil, false
	}
	return &c.items[i], true
}

// All returns every cosmetic in file order. Do not mutate.
func (c *Catalogue) All() []Cosmetic { return c.items }

// AllSeries returns the series in file order. Do not mutate.
func (c *Catalogue) AllSeries() []Series { return c.series }

// HasSlug reports whether a slug is used (faction_works.yaml decorations must not collide: shared art slots).
func (c *Catalogue) HasSlug(slug string) bool { return c.slugs[slug] }

// ShopTotal is the price of every shop cosmetic (balancesim stock).
func (c *Catalogue) ShopTotal() int64 {
	var sum int64
	for _, cm := range c.items {
		sum += cm.Price
	}
	return sum
}

// PerkGrant is one vip.yaml perk string and the tier whose `perks` list carries it.
type PerkGrant struct {
	Perk string
	Tier int
}

// EraIndex resolves world era ids to their order (world.Catalogue).
type EraIndex interface {
	EraSeq(id string) (int, bool)
}

// BindPerks cross-checks vip.yaml perks with the catalogue (decisions 2026-11-07): every perk whose key is a cosmetic
// kind resolves to a tien_duyen cosmetic, and every tien_duyen cosmetic is granted by exactly one tier. It records the
// granting tier (PerkTier). Called once by content.LoadAll before the catalogue is shared.
func (c *Catalogue) BindPerks(grants []PerkGrant) error {
	var errs []error
	granted := map[string]bool{}
	for _, g := range grants {
		perk, tier := g.Perk, g.Tier
		key, _, ok := strings.Cut(perk, ":")
		if !ok || !slices.Contains(Kinds, Kind(key)) {
			continue // breakthrough_fx, alchemy_slot … are not cosmetics of this catalogue
		}
		i, ok := c.byID[perk]
		if !ok {
			errs = append(errs, fmt.Errorf("vip.yaml perk %q (tier %d) has no cosmetics.yaml entry", perk, tier))
			continue
		}
		if c.items[i].Source != SourceTienDuyen {
			errs = append(errs, fmt.Errorf("vip.yaml perk %q points to a %s cosmetic; perks must be source tien_duyen", perk, c.items[i].Source))
			continue
		}
		if granted[perk] {
			errs = append(errs, fmt.Errorf("vip.yaml perk %q is granted by more than one tier", perk))
			continue
		}
		if tier < 1 {
			errs = append(errs, fmt.Errorf("vip.yaml perk %q on tier %d: cosmetic perks need tier ≥ 1 (tier 0 is everyone)", perk, tier))
			continue
		}
		granted[perk] = true
		c.items[i].PerkTier = tier
	}
	for _, cm := range c.items {
		if cm.Source == SourceTienDuyen && !granted[cm.ID] {
			errs = append(errs, fmt.Errorf("cosmetic %q is source tien_duyen but no vip.yaml tier grants it", cm.ID))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("cosmetic: perk cross-check: %w", errors.Join(errs...))
	}
	return nil
}

// CrossCheck validates era windows against the world catalogue and min_tier against the last Tiên Duyên level.
func (c *Catalogue) CrossCheck(eras EraIndex, maxTier int) error {
	var errs []error
	for _, s := range c.series {
		from, okF := 0, true
		if s.FromEra != "" {
			from, okF = eras.EraSeq(s.FromEra)
			if !okF {
				errs = append(errs, fmt.Errorf("cosmetic series %q: unknown from_era %q", s.ID, s.FromEra))
			}
		}
		if s.UntilEra != "" {
			until, ok := eras.EraSeq(s.UntilEra)
			if !ok {
				errs = append(errs, fmt.Errorf("cosmetic series %q: unknown until_era %q", s.ID, s.UntilEra))
			} else if okF && until < from {
				errs = append(errs, fmt.Errorf("cosmetic series %q: until_era before from_era", s.ID))
			}
		}
	}
	for _, cm := range c.items {
		if cm.MinTier > maxTier {
			errs = append(errs, fmt.Errorf("cosmetic %q: min_tier %d above the last Tiên Duyên level %d", cm.ID, cm.MinTier, maxTier))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("cosmetic: cross-reference errors: %w", errors.Join(errs...))
	}
	return nil
}

// ───── pure rules ─────

// Domain errors (mapped to HTTP in httpapi).
var (
	ErrUnknown      = errors.New("cosmetic: unknown cosmetic")
	ErrNotForSale   = errors.New("cosmetic: not sold (Tiên Duyên perk)")
	ErrUnavailable  = errors.New("cosmetic: outside its season/era window")
	ErrTierLocked   = errors.New("cosmetic: Tiên Duyên level too low")
	ErrAlreadyOwned = errors.New("cosmetic: already owned")
	ErrInsufficient = errors.New("cosmetic: not enough spirit stones")
	ErrNotOwned     = errors.New("cosmetic: not owned")
	ErrBadLoadout   = errors.New("cosmetic: invalid loadout")
	ErrNoCharacter  = errors.New("cosmetic: character not found")
	ErrKeyReuse     = errors.New("cosmetic: idempotency key reused for another operation")
)

// When is the clock/world context availability is judged in: the game-day month and the world era order
// (eraSeq = 0 before launch, i.e. the first era applies).
type When struct {
	Month  time.Month
	EraSeq int
}

// SeasonOf returns the season id holding a month ("" when none).
func (c *Catalogue) SeasonOf(m time.Month) string {
	for _, s := range c.Seasons {
		if slices.Contains(s.Months, int(m)) {
			return s.ID
		}
	}
	return ""
}

// Available reports whether a shop cosmetic can be bought now (season and era window). Tiên Duyên cosmetics are
// never "available" for sale.
func (c *Catalogue) Available(cm *Cosmetic, w When, eras EraIndex) bool {
	if cm.Source != SourceShop {
		return false
	}
	if cm.Season != "" && c.SeasonOf(w.Month) != cm.Season {
		return false
	}
	if cm.FromEra != "" {
		if from, ok := eras.EraSeq(cm.FromEra); !ok || w.EraSeq < from {
			return false
		}
	}
	if cm.UntilEra != "" {
		if until, ok := eras.EraSeq(cm.UntilEra); !ok || w.EraSeq > until {
			return false
		}
	}
	return true
}

// Quote validates a purchase. Order: unknown → not for sale → owned → window → tier → stones. Pure.
func (c *Catalogue) Quote(id string, owned bool, tier int, balance int64, w When, eras EraIndex) (*Cosmetic, error) {
	cm, ok := c.Get(id)
	if !ok {
		return nil, ErrUnknown
	}
	switch {
	case cm.Source != SourceShop:
		return cm, ErrNotForSale
	case owned:
		return cm, ErrAlreadyOwned
	case !c.Available(cm, w, eras):
		return cm, ErrUnavailable
	case tier < cm.MinTier:
		return cm, ErrTierLocked
	case balance < cm.Price:
		return cm, ErrInsufficient
	}
	return cm, nil
}

// Owns reports ownership: a purchase row, or a Tiên Duyên perk whose tier the character's level reaches.
func Owns(cm *Cosmetic, purchased map[string]bool, tier int) bool {
	if cm.Source == SourceTienDuyen {
		return cm.PerkTier > 0 && tier >= cm.PerkTier
	}
	return purchased[cm.ID]
}

// Appearance is the equipped look (ids only; the client resolves names/visuals from GET /cosmetics/catalog).
type Appearance struct {
	TitleID           string   `json:"titleId,omitempty"`
	AvatarFrameID     string   `json:"avatarFrameId,omitempty"`
	ChatAuraID        string   `json:"chatAuraId,omitempty"`
	CaveDecorationIDs []string `json:"caveDecorationIds"`
}

// Equal compares two loadouts (idempotency-key reuse check).
func (a Appearance) Equal(b Appearance) bool {
	return a.TitleID == b.TitleID && a.AvatarFrameID == b.AvatarFrameID && a.ChatAuraID == b.ChatAuraID &&
		slices.Equal(a.CaveDecorationIDs, b.CaveDecorationIDs)
}

// Validate checks a requested loadout: each id has the slot's kind, the cave list fits the slots without duplicates,
// and everything is owned. Empty = unequip that slot. Pure.
func (c *Catalogue) Validate(a Appearance, owns func(*Cosmetic) bool) error {
	check := func(id string, k Kind) error {
		if id == "" {
			return nil
		}
		cm, ok := c.Get(id)
		if !ok || cm.Kind != k {
			return fmt.Errorf("%w: %q is not a %s", ErrBadLoadout, id, k)
		}
		if !owns(cm) {
			return fmt.Errorf("%w: %q", ErrNotOwned, id)
		}
		return nil
	}
	if err := errors.Join(check(a.TitleID, KindTitle), check(a.AvatarFrameID, KindAvatarFrame), check(a.ChatAuraID, KindChatAura)); err != nil {
		return err
	}
	if len(a.CaveDecorationIDs) > c.Settings.CaveSlots {
		return fmt.Errorf("%w: at most %d cave decorations", ErrBadLoadout, c.Settings.CaveSlots)
	}
	seen := map[string]bool{}
	for _, id := range a.CaveDecorationIDs {
		if seen[id] {
			return fmt.Errorf("%w: %q twice", ErrBadLoadout, id)
		}
		seen[id] = true
		if err := check(id, KindCave); err != nil {
			return err
		}
	}
	return nil
}

// Visible returns id when a character at Tiên Duyên level `tier` may show it, "" otherwise: a Tiên Duyên perk above
// the level (refund / admin revoke lowered it) or an id no longer in the catalogue is hidden. Defence in depth on every
// read that shows a look to others; vip also strips such perks from character_appearance when the level drops.
func (c *Catalogue) Visible(id string, tier int) string {
	if id == "" {
		return ""
	}
	cm, ok := c.Get(id)
	if !ok || (cm.Source == SourceTienDuyen && tier < cm.PerkTier) {
		return ""
	}
	return id
}

// VisibleLook filters a whole appearance with Visible.
func (c *Catalogue) VisibleLook(a Appearance, tier int) Appearance {
	out := Appearance{TitleID: c.Visible(a.TitleID, tier), AvatarFrameID: c.Visible(a.AvatarFrameID, tier),
		ChatAuraID: c.Visible(a.ChatAuraID, tier), CaveDecorationIDs: []string{}}
	for _, id := range a.CaveDecorationIDs {
		if c.Visible(id, tier) != "" {
			out.CaveDecorationIDs = append(out.CaveDecorationIDs, id)
		}
	}
	return out
}

// TierOf maps cumulative điểm duyên to the level (vip.Table.TierForPoints), injected so hooks stay vip-agnostic.
type TierOf func(points int64) int

// NameFxFilter is chat's hook: the chat aura snapshot at send time, hidden when the sender's level no longer grants it.
func (c *Catalogue) NameFxFilter(tierOf TierOf) func(fx string, vipPoints int64) string {
	return func(fx string, vipPoints int64) string { return c.Visible(fx, tierOf(vipPoints)) }
}

// BoardFilter is the leaderboard / faction-members hook for the frame and title shown next to a portrait.
func (c *Catalogue) BoardFilter(tierOf TierOf) func(frame, title string, vipPoints int64) (string, string) {
	return func(frame, title string, vipPoints int64) (string, string) {
		t := tierOf(vipPoints)
		return c.Visible(frame, t), c.Visible(title, t)
	}
}
