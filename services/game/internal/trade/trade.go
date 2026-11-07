// Package trade is trao đổi vật phẩm (Phase 5.2, plan §3.12, cot-truyen §5.5 as overridden by the owner decision
// "Luật trao đổi"): player-to-player gifts and two-sided swaps of linh thạch and plain materials, with daily caps,
// account/character age gates, a small stone fee (sink) and a full log.
//
// trade.go is the catalogue (content/catalog/trade.yaml) and the pure rules; service.go owns the transactions.
package trade

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"tiendao/game/internal/item"
)

// Asset kinds.
const (
	KindCurrency = "currency"
	KindItem     = "item"
)

// SpiritStone is the only tradable currency (premium is never tradable).
const SpiritStone = "spirit_stone"

// Relations between the two parties (the received-value cap depends on it).
const (
	RelStranger = "stranger"
	RelFaction  = "faction"
	RelDaoLu    = "dao_lu"
)

// Offer kinds.
const (
	KindGift = "gift"
	KindSwap = "swap"
)

// Asset is one tradable asset with its stone-equivalent value per unit.
type Asset struct {
	ID    string `yaml:"id" json:"id"`
	Kind  string `yaml:"kind" json:"kind"`
	Value int64  `yaml:"value" json:"value"`
}

// Fee is trade.yaml fee.
type Fee struct {
	StoneFeeBP  int64 `yaml:"stone_fee_bp"`
	MinStoneFee int64 `yaml:"min_stone_fee"`
}

// Offers is trade.yaml offers.
type Offers struct {
	ExpiryHours        int64 `yaml:"expiry_hours"`
	MaxLinesPerSide    int   `yaml:"max_lines_per_side"`
	MaxQuantityPerLine int64 `yaml:"max_quantity_per_line"`
	MaxOpenOutgoing    int32 `yaml:"max_open_outgoing"`
	ListLimit          int32 `yaml:"list_limit"`
	HistoryLimit       int32 `yaml:"history_limit"`
}

// ReceivedCaps is the received-value cap per relation.
type ReceivedCaps struct {
	Stranger int64 `yaml:"stranger" json:"stranger"`
	Faction  int64 `yaml:"faction" json:"faction"`
	DaoLu    int64 `yaml:"dao_lu" json:"daoLu"`
}

// Daily is trade.yaml daily.
type Daily struct {
	OffersPerDay        int32        `yaml:"offers_per_day"`
	CompletedPerDay     int32        `yaml:"completed_per_day"`
	SentValuePerDay     int64        `yaml:"sent_value_per_day"`
	ReceivedValuePerDay ReceivedCaps `yaml:"received_value_per_day"`
}

// Gate is one side's minimum account age, character age and realm tier.
type Gate struct {
	MinAccountAgeDays   int64 `yaml:"min_account_age_days" json:"minAccountAgeDays"`
	MinCharacterAgeDays int64 `yaml:"min_character_age_days" json:"minCharacterAgeDays"`
	MinTier             int   `yaml:"min_tier" json:"minTier"`
}

// Gates is trade.yaml gates.
type Gates struct {
	Send    Gate `yaml:"send"`
	Receive Gate `yaml:"receive"`
}

// Alerts is trade.yaml alerts (admin flow report thresholds).
type Alerts struct {
	WindowDays       int   `yaml:"window_days"`
	FunnelMinSenders int32 `yaml:"funnel_min_senders"`
	PairMinValue     int64 `yaml:"pair_min_value"`
	LopsidedMinBP    int64 `yaml:"lopsided_min_bp"`
	ListLimit        int32 `yaml:"list_limit"`
}

// Config is content/catalog/trade.yaml. Immutable after load.
type Config struct {
	Version int     `yaml:"version"`
	Assets  []Asset `yaml:"assets"`
	Fee     Fee     `yaml:"fee"`
	Offers  Offers  `yaml:"offers"`
	Daily   Daily   `yaml:"daily"`
	Gates   Gates   `yaml:"gates"`
	Alerts  Alerts  `yaml:"alerts"`

	byID map[string]*Asset
}

var slugRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// LoadFile parses and validates trade.yaml.
func LoadFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load parses and validates trade YAML (self-contained checks; item cross-checks are in CrossCheck).
func Load(r io.Reader) (*Config, error) {
	var c Config
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("trade: decode: %w", err)
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if c.Version != 1 {
		fail("unsupported version %d", c.Version)
	}
	c.byID = map[string]*Asset{}
	stone := false
	for i := range c.Assets {
		a := &c.Assets[i]
		if !slugRe.MatchString(a.ID) {
			fail("asset[%d]: invalid id %q", i, a.ID)
		}
		if _, dup := c.byID[a.ID]; dup {
			fail("asset %q: duplicate", a.ID)
		}
		switch a.Kind {
		case KindCurrency:
			if a.ID != SpiritStone { // premium (and any future currency) is never tradable
				fail("asset %q: the only tradable currency is %s", a.ID, SpiritStone)
			}
			stone = true
		case KindItem:
			if a.ID == SpiritStone {
				fail("asset %q must be kind currency", a.ID)
			}
		default:
			fail("asset %q: kind must be currency|item", a.ID)
		}
		if a.Value < 1 || a.Value > 1_000_000 {
			fail("asset %q: value must be within 1..1000000", a.ID)
		}
		c.byID[a.ID] = a
	}
	if !stone {
		fail("assets: %s must be listed", SpiritStone)
	}
	if c.Fee.StoneFeeBP < 0 || c.Fee.StoneFeeBP > 2000 {
		fail("fee.stone_fee_bp must be within 0..2000")
	}
	if c.Fee.MinStoneFee < 0 || c.Fee.MinStoneFee > 100 {
		fail("fee.min_stone_fee must be within 0..100")
	}
	o, d := c.Offers, c.Daily
	if o.ExpiryHours < 1 || o.ExpiryHours > 168 {
		fail("offers.expiry_hours must be within 1..168")
	}
	if o.MaxLinesPerSide < 1 || o.MaxLinesPerSide > 8 {
		fail("offers.max_lines_per_side must be within 1..8")
	}
	if o.MaxQuantityPerLine < 1 || o.MaxQuantityPerLine > 1_000_000 {
		fail("offers.max_quantity_per_line must be within 1..1000000")
	}
	if o.MaxOpenOutgoing < 1 || o.MaxOpenOutgoing > 20 {
		fail("offers.max_open_outgoing must be within 1..20")
	}
	if o.ListLimit < 1 || o.ListLimit > 100 || o.HistoryLimit < 1 || o.HistoryLimit > 200 {
		fail("offers.list_limit must be within 1..100 and history_limit within 1..200")
	}
	if d.OffersPerDay < 1 || d.OffersPerDay > 50 {
		fail("daily.offers_per_day must be within 1..50")
	}
	if d.CompletedPerDay < 1 || d.CompletedPerDay > 50 {
		fail("daily.completed_per_day must be within 1..50")
	}
	rc := d.ReceivedValuePerDay
	if rc.Stranger < 1 || rc.Stranger > rc.Faction || rc.Faction > rc.DaoLu || rc.DaoLu > 100_000 {
		fail("daily.received_value_per_day must satisfy 1 ≤ stranger ≤ faction ≤ dao_lu ≤ 100000")
	}
	if d.SentValuePerDay < rc.Stranger || d.SentValuePerDay > 1_000_000 {
		fail("daily.sent_value_per_day must be within received_value_per_day.stranger..1000000")
	}
	for name, g := range map[string]Gate{"send": c.Gates.Send, "receive": c.Gates.Receive} {
		if g.MinAccountAgeDays < 0 || g.MinAccountAgeDays > 180 || g.MinCharacterAgeDays < 0 || g.MinCharacterAgeDays > 180 {
			fail("gates.%s ages must be within 0..180 days", name)
		}
		if g.MinTier < 1 || g.MinTier > 20 {
			fail("gates.%s.min_tier must be within 1..20", name)
		}
	}
	// Owner decision: new accounts AND new characters cannot receive.
	if c.Gates.Receive.MinAccountAgeDays < 1 || c.Gates.Receive.MinCharacterAgeDays < 1 {
		fail("gates.receive: min_account_age_days and min_character_age_days must be ≥ 1 (new accounts cannot receive)")
	}
	a := c.Alerts
	if a.WindowDays < 1 || a.WindowDays > 30 || a.FunnelMinSenders < 2 || a.PairMinValue < 1 ||
		a.LopsidedMinBP < 5000 || a.LopsidedMinBP > 10000 || a.ListLimit < 1 || a.ListLimit > 200 {
		fail("alerts: window_days 1..30, funnel_min_senders ≥ 2, pair_min_value ≥ 1, lopsided_min_bp 5000..10000, list_limit 1..200")
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("trade: invalid config: %w", errors.Join(errs...))
	}
	return &c, nil
}

// Asset returns a tradable asset (false = not tradable).
func (c *Config) Asset(id string) (*Asset, bool) { a, ok := c.byID[id]; return a, ok }

// Expiry is an offer's lifetime.
func (c *Config) Expiry() time.Duration { return time.Duration(c.Offers.ExpiryHours) * time.Hour }

// ForbiddenItems are item ids that must never be tradable, with the reason (built by content.LoadAll from the other
// catalogues: breakthrough requirement items, shop listings).
type ForbiddenItems map[string]string

// CrossCheck validates the asset list against items.yaml. An item asset must exist, be `tradable: true`, be a plain
// material (category herb|material|fragment, no effect field, no odds) and appear in no forbidden list; and every
// `tradable: true` item must be listed (so the flag shown by the UI equals what the server accepts).
func (c *Config) CrossCheck(items *item.Catalogue, forbidden ForbiddenItems) error {
	var errs []error
	plain := map[item.Category]bool{"herb": true, "material": true, "fragment": true}
	for _, a := range c.Assets {
		if a.Kind != KindItem {
			continue
		}
		d, ok := items.Get(a.ID)
		switch {
		case !ok:
			errs = append(errs, fmt.Errorf("trade asset %q: unknown item", a.ID))
			continue
		case !d.Tradable:
			errs = append(errs, fmt.Errorf("trade asset %q: items.yaml tradable is false", a.ID))
		case d.Category == item.CategoryArtifact:
			errs = append(errs, fmt.Errorf("trade asset %q: pháp bảo are never tradable (tribulation odds)", a.ID))
		case d.Category == "cosmetic":
			errs = append(errs, fmt.Errorf("trade asset %q: cosmetics are never tradable", a.ID))
		case !plain[d.Category]:
			errs = append(errs, fmt.Errorf("trade asset %q: only plain materials (herb|material|fragment) are tradable, got %s", a.ID, d.Category))
		}
		if d.RaisesOdds() || len(d.EffectKeys()) > 0 || d.UniquePerCharacter {
			errs = append(errs, fmt.Errorf("trade asset %q: items with effects/odds are never tradable", a.ID))
		}
		if why, bad := forbidden[a.ID]; bad {
			errs = append(errs, fmt.Errorf("trade asset %q: never tradable (%s)", a.ID, why))
		}
	}
	for _, d := range items.All() {
		if d.Tradable {
			if _, ok := c.byID[d.ID]; !ok {
				errs = append(errs, fmt.Errorf("item %q is tradable: true but has no trade.yaml asset (value) — list it or set tradable: false", d.ID))
			}
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// ───── pure rules ─────

// Line is one requested movement (input).
type Line struct {
	AssetID  string `json:"assetId"`
	Quantity int64  `json:"quantity"`
}

// PricedLine is a validated line with its asset.
type PricedLine struct {
	AssetID   string
	Kind      string
	Quantity  int64
	UnitValue int64
}

// Value is the stone-equivalent of the line.
func (l PricedLine) Value() int64 { return l.Quantity * l.UnitValue }

// Plan is a validated offer: give = sender → receiver, take = receiver → sender.
type Plan struct {
	Kind string
	Give []PricedLine
	Take []PricedLine
}

func sum(ls []PricedLine, f func(PricedLine) int64) int64 {
	var s int64
	for _, l := range ls {
		s += f(l)
	}
	return s
}

func stonesOf(l PricedLine) int64 {
	if l.Kind == KindCurrency {
		return l.Quantity
	}
	return 0
}

// ValueToReceiver / ValueToSender are the stone-equivalents each party receives.
func (p Plan) ValueToReceiver() int64 { return sum(p.Give, PricedLine.Value) }
func (p Plan) ValueToSender() int64   { return sum(p.Take, PricedLine.Value) }

// StonesGiven / StonesTaken are the linh thạch moved each way (fees excluded).
func (p Plan) StonesGiven() int64 { return sum(p.Give, stonesOf) }
func (p Plan) StonesTaken() int64 { return sum(p.Take, stonesOf) }

// ItemIDs lists the item assets of both sides (sorted by the caller when locking).
func (p Plan) ItemIDs() []string {
	var ids []string
	for _, l := range append(append([]PricedLine{}, p.Give...), p.Take...) {
		if l.Kind == KindItem {
			ids = append(ids, l.AssetID)
		}
	}
	return ids
}

// LineError is a malformed or forbidden offer (HTTP 400). Code: empty_gift | too_many_lines | bad_quantity |
// duplicate_line | not_tradable | same_asset_both_sides.
type LineError struct {
	Code    string
	AssetID string
}

func (e *LineError) Error() string {
	if e.AssetID != "" {
		return fmt.Sprintf("trade: %s (%s)", e.Code, e.AssetID)
	}
	return "trade: " + e.Code
}

// Validate turns raw lines into a plan. Anything not in trade.yaml (artifacts, requirement items, shop items,
// cosmetics, premium currency, unknown ids) is refused here, whatever the request says. Pure.
func (c *Config) Validate(give, take []Line) (Plan, error) {
	if len(give) == 0 {
		return Plan{}, &LineError{Code: "empty_gift"}
	}
	if len(give) > c.Offers.MaxLinesPerSide || len(take) > c.Offers.MaxLinesPerSide {
		return Plan{}, &LineError{Code: "too_many_lines"}
	}
	side := func(ls []Line, seen map[string]bool) ([]PricedLine, error) {
		out := make([]PricedLine, 0, len(ls))
		for _, l := range ls {
			id := strings.TrimSpace(l.AssetID)
			a, ok := c.byID[id]
			if !ok {
				return nil, &LineError{Code: "not_tradable", AssetID: id}
			}
			if l.Quantity < 1 || l.Quantity > c.Offers.MaxQuantityPerLine {
				return nil, &LineError{Code: "bad_quantity", AssetID: id}
			}
			if seen[id] {
				return nil, &LineError{Code: "duplicate_line", AssetID: id}
			}
			seen[id] = true
			out = append(out, PricedLine{AssetID: id, Kind: a.Kind, Quantity: l.Quantity, UnitValue: a.Value})
		}
		return out, nil
	}
	g, err := side(give, map[string]bool{})
	if err != nil {
		return Plan{}, err
	}
	gs := map[string]bool{}
	for _, l := range g {
		gs[l.AssetID] = true
	}
	takeSeen := map[string]bool{}
	t, err := side(take, takeSeen)
	if err != nil {
		return Plan{}, err
	}
	for id := range takeSeen {
		if gs[id] { // stones for stones, linh thảo for linh thảo: meaningless and confusing (one ledger key per side)
			return Plan{}, &LineError{Code: "same_asset_both_sides", AssetID: id}
		}
	}
	p := Plan{Kind: KindGift, Give: g, Take: t}
	if len(t) > 0 {
		p.Kind = KindSwap
	}
	return p, nil
}

// StoneFee is the linh thạch burned when `stones` are given: ceil(stones × fee_bp / 10000), at least min_stone_fee; 0 for 0.
func (c *Config) StoneFee(stones int64) int64 {
	if stones <= 0 {
		return 0
	}
	f := (stones*c.Fee.StoneFeeBP + 9_999) / 10_000
	return max(f, c.Fee.MinStoneFee)
}

// ReceiveCap is the daily received-value cap for a relation.
func (c *Config) ReceiveCap(rel string) int64 {
	switch rel {
	case RelDaoLu:
		return c.Daily.ReceivedValuePerDay.DaoLu
	case RelFaction:
		return c.Daily.ReceivedValuePerDay.Faction
	default:
		return c.Daily.ReceivedValuePerDay.Stranger
	}
}

// Relation picks the strongest relation (đạo lữ > same faction > stranger).
func Relation(daoLu bool, factionA, factionB string) string {
	switch {
	case daoLu:
		return RelDaoLu
	case factionA != "" && factionA == factionB:
		return RelFaction
	default:
		return RelStranger
	}
}

// Gate reasons (API `blockers` codes).
const (
	BlockAccountAge   = "account_age"
	BlockCharacterAge = "character_age"
	BlockTier         = "tier"
)

// Party is what the gates need about one side.
type Party struct {
	AccountCreatedAt   time.Time
	CharacterCreatedAt time.Time
	Tier               int
}

const day = 24 * time.Hour

func blockers(g Gate, p Party, now time.Time) []string {
	out := []string{}
	if now.Sub(p.AccountCreatedAt) < time.Duration(g.MinAccountAgeDays)*day {
		out = append(out, BlockAccountAge)
	}
	if now.Sub(p.CharacterCreatedAt) < time.Duration(g.MinCharacterAgeDays)*day {
		out = append(out, BlockCharacterAge)
	}
	if p.Tier < g.MinTier {
		out = append(out, BlockTier)
	}
	return out
}

// SendBlockers / ReceiveBlockers list why a party may not give / receive now (empty = may). Pure.
func (c *Config) SendBlockers(p Party, now time.Time) []string { return blockers(c.Gates.Send, p, now) }
func (c *Config) ReceiveBlockers(p Party, now time.Time) []string {
	return blockers(c.Gates.Receive, p, now)
}

// Day is one character's counters for a game day (trade_days).
type Day struct {
	OffersCreated int32
	Completed     int32
	ValueReceived int64
	ValueSent     int64
}

// Limit codes (LimitError.Code, HTTP 409/429).
const (
	LimitOffersPerDay = "offers_per_day"
	LimitCompleted    = "completed_per_day"
	LimitReceived     = "received_value_cap"
	LimitSent         = "sent_value_cap"
)

// LimitError is a daily limit a party would exceed. Side is "sender" or "receiver".
type LimitError struct {
	Side      string
	Code      string
	Cap       int64
	Used      int64
	Requested int64
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("trade: %s %s: used %d + %d > cap %d", e.Side, e.Code, e.Used, e.Requested, e.Cap)
}

// CheckAccept verifies both parties' daily limits for completing the plan between them (rel = relation now). Pure.
// The receiver receives ValueToReceiver and gives ValueToSender; the sender the reverse.
func (c *Config) CheckAccept(p Plan, rel string, sender, receiver Day) error {
	capRecv := c.ReceiveCap(rel)
	for _, x := range []struct {
		side       string
		d          Day
		recv, sent int64
	}{
		{"sender", sender, p.ValueToSender(), p.ValueToReceiver()},
		{"receiver", receiver, p.ValueToReceiver(), p.ValueToSender()},
	} {
		if x.d.Completed >= c.Daily.CompletedPerDay {
			return &LimitError{Side: x.side, Code: LimitCompleted, Cap: int64(c.Daily.CompletedPerDay), Used: int64(x.d.Completed), Requested: 1}
		}
		if x.recv > 0 && x.d.ValueReceived+x.recv > capRecv {
			return &LimitError{Side: x.side, Code: LimitReceived, Cap: capRecv, Used: x.d.ValueReceived, Requested: x.recv}
		}
		if x.sent > 0 && x.d.ValueSent+x.sent > c.Daily.SentValuePerDay {
			return &LimitError{Side: x.side, Code: LimitSent, Cap: c.Daily.SentValuePerDay, Used: x.d.ValueSent, Requested: x.sent}
		}
	}
	return nil
}

// CheckCreate is the sender-side check at creation: today's offer count and the static caps (an offer that could
// never be accepted under the relation's caps is refused up front). Pure.
func (c *Config) CheckCreate(p Plan, rel string, sender Day) error {
	if sender.OffersCreated >= c.Daily.OffersPerDay {
		return &LimitError{Side: "sender", Code: LimitOffersPerDay, Cap: int64(c.Daily.OffersPerDay), Used: int64(sender.OffersCreated), Requested: 1}
	}
	return c.CheckAccept(p, rel, Day{}, Day{})
}
