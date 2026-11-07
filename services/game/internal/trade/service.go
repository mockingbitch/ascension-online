package trade

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

	"tiendao/game/internal/db"
	"tiendao/game/internal/economy"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/item"
	"tiendao/game/internal/netsig"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/realm"
)

// Domain errors (mapped to HTTP in httpapi).
var (
	ErrNoCharacter     = errors.New("trade: character not found")
	ErrTargetNotFound  = errors.New("trade: target character not found")
	ErrSelf            = errors.New("trade: cannot trade with yourself")
	ErrNotAcknowledged = errors.New("trade: the irreversible-transfer warning must be acknowledged")
	ErrOfferGone       = errors.New("trade: offer expired, answered or not found")
	ErrKeyReused       = errors.New("trade: idempotency key reused for another request")
	ErrTooManyOpen     = errors.New("trade: too many open offers")
)

// GateError: a party fails trade.yaml gates. Side is relative to the caller ("self" | "other"); Action "send" | "receive".
type GateError struct {
	Side     string
	Action   string
	Blockers []string
}

func (e *GateError) Error() string {
	return fmt.Sprintf("trade: %s cannot %s now: %s", e.Side, e.Action, strings.Join(e.Blockers, ","))
}

// InsufficientError: a party no longer holds what the offer moves (Side relative to the caller).
type InsufficientError struct {
	Side    string
	AssetID string
}

func (e *InsufficientError) Error() string {
	return fmt.Sprintf("trade: %s lacks %s", e.Side, e.AssetID)
}

// Ledger reasons (currency_ledger / inventory_transactions), ref_type "trade", ref_id = offer id.
const (
	ReasonSend    = "trade_send"
	ReasonReceive = "trade_receive"
	ReasonFee     = "trade_fee"
)

// Service owns trade transactions.
type Service struct {
	Pool   *pgxpool.Pool
	Cfg    *Config
	Items  *item.Catalogue
	Realms *realm.Catalogue
	Net    *netsig.Store // shared-network signal; nil → never flagged
	Now    func() time.Time
	Log    *slog.Logger
}

// ───── views ─────

// LineView is one line with its current stone-equivalent unit value.
type LineView struct {
	AssetID   string `json:"assetId"`
	Kind      string `json:"kind"`
	Quantity  int64  `json:"quantity"`
	UnitValue int64  `json:"unitValue"`
}

// PartyView is the other side of an offer.
type PartyView struct {
	CharacterID string `json:"characterId"`
	Name        string `json:"name"`
	StageID     string `json:"stageId"`
}

// OfferView is one offer as the viewer sees it. give = sender → receiver, take = receiver → sender.
// Pending offers show what accepting NOW would move and burn; answered ones what was frozen at accept.
type OfferView struct {
	ID              uuid.UUID  `json:"id"`
	Kind            string     `json:"kind"`
	Status          string     `json:"status"`    // pending | accepted | declined | cancelled | expired
	Direction       string     `json:"direction"` // incoming (I am the receiver) | outgoing
	Counterpart     PartyView  `json:"counterpart"`
	Give            []LineView `json:"give"`
	Take            []LineView `json:"take"`
	ValueToReceiver int64      `json:"valueToReceiver"`
	ValueToSender   int64      `json:"valueToSender"`
	SenderFee       int64      `json:"senderFee"`
	ReceiverFee     int64      `json:"receiverFee"`
	Relation        string     `json:"relation"`
	CreatedAt       time.Time  `json:"createdAt"`
	ExpiresAt       time.Time  `json:"expiresAt"`
	RespondedAt     *time.Time `json:"respondedAt"`
}

// RulesView is the catalogue part the client shows (numbers never hardcoded in TS).
type RulesView struct {
	Assets              []Asset      `json:"assets"`
	StoneFeeBP          int64        `json:"stoneFeeBp"`
	MinStoneFee         int64        `json:"minStoneFee"`
	ExpiryHours         int64        `json:"expiryHours"`
	MaxLinesPerSide     int          `json:"maxLinesPerSide"`
	MaxQuantityPerLine  int64        `json:"maxQuantityPerLine"`
	MaxOpenOutgoing     int32        `json:"maxOpenOutgoing"`
	OffersPerDay        int32        `json:"offersPerDay"`
	CompletedPerDay     int32        `json:"completedPerDay"`
	SentValuePerDay     int64        `json:"sentValuePerDay"`
	ReceivedValuePerDay ReceivedCaps `json:"receivedValuePerDay"`
	Send                Gate         `json:"send"`
	Receive             Gate         `json:"receive"`
}

// LimitsView is the viewer's usage today.
type LimitsView struct {
	GameDay         string    `json:"gameDay"`
	ResetAt         time.Time `json:"resetAt"`
	OffersLeft      int32     `json:"offersLeft"`
	CompletedLeft   int32     `json:"completedLeft"`
	OpenOutgoing    int32     `json:"openOutgoing"`
	ValueReceived   int64     `json:"valueReceived"`
	ValueSent       int64     `json:"valueSent"`
	SendBlockers    []string  `json:"sendBlockers"`
	ReceiveBlockers []string  `json:"receiveBlockers"`
}

// View is GET /trades and the answer to every trade mutation.
type View struct {
	ServerNow time.Time   `json:"serverNow"`
	Rules     RulesView   `json:"rules"`
	Limits    LimitsView  `json:"limits"`
	Incoming  []OfferView `json:"incoming"`
	Outgoing  []OfferView `json:"outgoing"`
	Replayed  bool        `json:"replayed,omitempty"` // an idempotent replay of a create/accept/decline/cancel (nothing changed)
}

// PeerView is GET /trades/peer/{characterId}: what the composer needs about a recipient (no usage of theirs is shown).
type PeerView struct {
	PartyView
	Relation   string   `json:"relation"`
	ReceiveCap int64    `json:"receiveCap"` // daily received-value cap between the two (by relation)
	Blockers   []string `json:"blockers"`   // why they cannot receive now
}

func (s *Service) rules() RulesView {
	c := s.Cfg
	return RulesView{Assets: c.Assets, StoneFeeBP: c.Fee.StoneFeeBP, MinStoneFee: c.Fee.MinStoneFee, ExpiryHours: c.Offers.ExpiryHours,
		MaxLinesPerSide: c.Offers.MaxLinesPerSide, MaxQuantityPerLine: c.Offers.MaxQuantityPerLine, MaxOpenOutgoing: c.Offers.MaxOpenOutgoing,
		OffersPerDay: c.Daily.OffersPerDay, CompletedPerDay: c.Daily.CompletedPerDay, SentValuePerDay: c.Daily.SentValuePerDay,
		ReceivedValuePerDay: c.Daily.ReceivedValuePerDay, Send: c.Gates.Send, Receive: c.Gates.Receive}
}

func (s *Service) me(ctx context.Context, q *db.Queries, userID uuid.UUID) (db.Character, error) {
	ch, err := q.GetCharacterByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ch, ErrNoCharacter
	}
	return ch, err
}

// party loads one side's gate inputs.
func (s *Service) party(ctx context.Context, q *db.Queries, charID uuid.UUID) (db.GetTradePartyRow, Party, error) {
	row, err := q.GetTradeParty(ctx, charID)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, Party{}, ErrTargetNotFound
	} else if err != nil {
		return row, Party{}, err
	}
	p := Party{AccountCreatedAt: row.AccountCreatedAt, CharacterCreatedAt: row.CreatedAt}
	if st, err := s.Realms.Stage(row.StageID); err == nil {
		p.Tier = st.Tier
	}
	return row, p, nil
}

func (s *Service) relation(ctx context.Context, q *db.Queries, a, b db.GetTradePartyRow) (string, error) {
	bonded, err := q.TradeDaoLuBonded(ctx, db.TradeDaoLuBondedParams{A: a.ID, B: b.ID})
	if err != nil {
		return "", err
	}
	return Relation(bonded, a.FactionID, b.FactionID), nil
}

func side(isCaller bool) string {
	if isCaller {
		return "self"
	}
	return "other"
}

// gates checks both parties for what the plan makes them do (give → send gate, receive → receive gate).
func (s *Service) gates(p Plan, sender, receiver Party, callerIsSender bool, now time.Time) error {
	type chk struct {
		party  Party
		caller bool
		action string
	}
	checks := []chk{{sender, callerIsSender, "send"}, {receiver, !callerIsSender, "receive"}}
	if p.Kind == KindSwap {
		checks = append(checks, chk{receiver, !callerIsSender, "send"}, chk{sender, callerIsSender, "receive"})
	}
	for _, c := range checks {
		var b []string
		if c.action == "send" {
			b = s.Cfg.SendBlockers(c.party, now)
		} else {
			b = s.Cfg.ReceiveBlockers(c.party, now)
		}
		if len(b) > 0 {
			return &GateError{Side: side(c.caller), Action: c.action, Blockers: b}
		}
	}
	return nil
}

// relativeLimit rewrites a LimitError's sender/receiver side relative to the caller.
func relativeLimit(err error, callerIsSender bool) error {
	var le *LimitError
	if errors.As(err, &le) {
		c := *le
		c.Side = side((c.Side == "sender") == callerIsSender)
		return &c
	}
	return err
}

// plan validates lines against trade.yaml AND items.yaml at call time (belt and braces: a crafted request or a later
// catalogue edit can never move an artifact, a requirement item, a cosmetic or a non-tradable item).
func (s *Service) plan(give, take []Line) (Plan, error) {
	p, err := s.Cfg.Validate(give, take)
	if err != nil {
		return p, err
	}
	for _, id := range p.ItemIDs() {
		d, ok := s.Items.Get(id)
		if !ok || !d.Tradable || d.Category == item.CategoryArtifact || d.Category == "cosmetic" || d.RaisesOdds() || len(d.EffectKeys()) > 0 {
			return Plan{}, &LineError{Code: "not_tradable", AssetID: id}
		}
	}
	return p, nil
}

func linesOf(rows []db.TradeLine, sideName string) []Line {
	var out []Line
	for _, r := range rows {
		if r.Side == sideName {
			out = append(out, Line{AssetID: r.AssetID, Quantity: r.Quantity})
		}
	}
	return out
}

func dayOf(d db.TradeDay) Day {
	return Day{OffersCreated: d.OffersCreated, Completed: d.Completed, ValueReceived: d.ValueReceived, ValueSent: d.ValueSent}
}

func uuidPG(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func event(ctx context.Context, q *db.Queries, offer uuid.UUID, kind string, actor uuid.UUID, detail map[string]any, now time.Time) error {
	var raw []byte
	if detail != nil {
		raw, _ = json.Marshal(detail)
	}
	a := pgtype.UUID{}
	if actor != uuid.Nil {
		a = uuidPG(actor)
	}
	return q.InsertTradeEvent(ctx, db.InsertTradeEventParams{OfferID: offer, Kind: kind, ActorCharacterID: a, Detail: raw, CreatedAt: now})
}

// holds reports whether the character currently holds the lines plus the stone fee (soft check, no lock).
func holds(ctx context.Context, q *db.Queries, charID uuid.UUID, lines []PricedLine, fee int64) (string, error) {
	var ids []string
	for _, l := range lines {
		if l.Kind == KindItem {
			ids = append(ids, l.AssetID)
		}
	}
	inv := map[string]int64{}
	if len(ids) > 0 {
		rows, err := q.GetInventoryItems(ctx, db.GetInventoryItemsParams{CharacterID: charID, ItemIds: ids})
		if err != nil {
			return "", err
		}
		for _, r := range rows {
			inv[r.ItemID] = r.Quantity
		}
	}
	var stones int64
	if bals, err := q.ListCurrencyBalances(ctx, charID); err != nil {
		return "", err
	} else {
		for _, b := range bals {
			if b.Currency == SpiritStone {
				stones = b.Balance
			}
		}
	}
	for _, l := range lines {
		if l.Kind == KindCurrency && stones < l.Quantity+fee {
			return l.AssetID, nil
		}
		if l.Kind == KindItem && inv[l.AssetID] < l.Quantity {
			return l.AssetID, nil
		}
	}
	return "", nil
}

// sameOffer reports whether the sender already created an offer with this key (true = replay). The key reused for a
// different receiver or different lines is ErrKeyReused (an edited offer resent with the old key is never silently
// dropped).
func sameOffer(ctx context.Context, q *db.Queries, sender, receiver uuid.UUID, p Plan, idemKey string) (bool, error) {
	prev, err := q.GetTradeOfferByKey(ctx, db.GetTradeOfferByKeyParams{SenderCharacterID: sender, IdempotencyKey: idemKey})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if prev.ReceiverCharacterID != receiver {
		return false, ErrKeyReused
	}
	rows, err := q.ListTradeLines(ctx, []uuid.UUID{prev.ID})
	if err != nil {
		return false, err
	}
	want := map[string]int64{}
	for _, l := range p.Give {
		want["give:"+l.AssetID] = l.Quantity
	}
	for _, l := range p.Take {
		want["take:"+l.AssetID] = l.Quantity
	}
	if len(rows) != len(want) {
		return false, ErrKeyReused
	}
	for _, r := range rows {
		if want[r.Side+":"+r.AssetID] != r.Quantity {
			return false, ErrKeyReused
		}
	}
	return true, nil
}

// ───── reads ─────

// State is GET /trades.
func (s *Service) State(ctx context.Context, userID uuid.UUID) (View, error) {
	q := db.New(s.Pool)
	ch, err := s.me(ctx, q, userID)
	if err != nil {
		return View{}, err
	}
	return s.view(ctx, q, ch.ID)
}

func (s *Service) view(ctx context.Context, q *db.Queries, charID uuid.UUID) (View, error) {
	now := s.Now().UTC()
	v := View{ServerNow: now, Rules: s.rules(), Incoming: []OfferView{}, Outgoing: []OfferView{}}
	_, me, err := s.party(ctx, q, charID)
	if err != nil {
		return v, err
	}
	d, err := q.GetTradeDay(ctx, db.GetTradeDayParams{CharacterID: charID, GameDay: gameday.PG(now)})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return v, err
	}
	open, err := q.CountOpenTradeOffers(ctx, db.CountOpenTradeOffersParams{CharacterID: charID, Now: now})
	if err != nil {
		return v, err
	}
	v.Limits = LimitsView{GameDay: gameday.Of(now).Format(time.DateOnly), ResetAt: gameday.NextReset(now),
		OffersLeft: max(0, s.Cfg.Daily.OffersPerDay-d.OffersCreated), CompletedLeft: max(0, s.Cfg.Daily.CompletedPerDay-d.Completed),
		OpenOutgoing: open, ValueReceived: d.ValueReceived, ValueSent: d.ValueSent,
		SendBlockers: s.Cfg.SendBlockers(me, now), ReceiveBlockers: s.Cfg.ReceiveBlockers(me, now)}
	for _, incoming := range []bool{true, false} {
		rows, err := q.ListTradeOffersOpen(ctx, db.ListTradeOffersOpenParams{Incoming: incoming, Now: now, CharacterID: charID, Lim: s.Cfg.Offers.ListLimit})
		if err != nil {
			return v, err
		}
		offers := make([]offerRow, len(rows))
		for i, r := range rows {
			offers[i] = offerRow{o: db.TradeOffer{ID: r.ID, Kind: r.Kind, SenderCharacterID: r.SenderCharacterID, ReceiverCharacterID: r.ReceiverCharacterID,
				Status: r.Status, Relation: r.Relation, ValueToReceiver: r.ValueToReceiver, ValueToSender: r.ValueToSender, SenderFee: r.SenderFee,
				ReceiverFee: r.ReceiverFee, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, RespondedAt: r.RespondedAt}, name: r.CounterpartName, stage: r.CounterpartStageID}
		}
		views, err := s.offerViews(ctx, q, charID, offers, now)
		if err != nil {
			return v, err
		}
		if incoming {
			v.Incoming = views
		} else {
			v.Outgoing = views
		}
	}
	return v, nil
}

type offerRow struct {
	o     db.TradeOffer
	name  string
	stage string
}

func (s *Service) offerViews(ctx context.Context, q *db.Queries, me uuid.UUID, rows []offerRow, now time.Time) ([]OfferView, error) {
	out := make([]OfferView, 0, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.o.ID
	}
	lines, err := q.ListTradeLines(ctx, ids)
	if err != nil {
		return nil, err
	}
	byOffer := map[uuid.UUID][]db.TradeLine{}
	for _, l := range lines {
		byOffer[l.OfferID] = append(byOffer[l.OfferID], l)
	}
	for _, r := range rows {
		o := r.o
		ov := OfferView{ID: o.ID, Kind: o.Kind, Status: o.Status, Direction: "outgoing", Give: []LineView{}, Take: []LineView{},
			Counterpart: PartyView{Name: r.name, StageID: r.stage}, ValueToReceiver: o.ValueToReceiver, ValueToSender: o.ValueToSender,
			SenderFee: o.SenderFee, ReceiverFee: o.ReceiverFee, Relation: o.Relation, CreatedAt: o.CreatedAt, ExpiresAt: o.ExpiresAt,
			RespondedAt: o.RespondedAt}
		if o.ReceiverCharacterID == me {
			ov.Direction = "incoming"
			ov.Counterpart.CharacterID = o.SenderCharacterID.String()
		} else {
			ov.Counterpart.CharacterID = o.ReceiverCharacterID.String()
		}
		var giveStones, takeStones int64
		for _, l := range byOffer[o.ID] {
			lv := LineView{AssetID: l.AssetID, Kind: l.AssetKind, Quantity: l.Quantity}
			if a, ok := s.Cfg.Asset(l.AssetID); ok {
				lv.UnitValue = a.Value
			}
			if l.Side == "give" {
				ov.Give = append(ov.Give, lv)
				if l.AssetKind == KindCurrency {
					giveStones += l.Quantity
				}
			} else {
				ov.Take = append(ov.Take, lv)
				if l.AssetKind == KindCurrency {
					takeStones += l.Quantity
				}
			}
		}
		if o.Status == "pending" {
			if !now.Before(o.ExpiresAt) {
				ov.Status = "expired" // lapsed; the worker records it
			}
			// What accepting now would move and burn (current catalogue).
			ov.SenderFee, ov.ReceiverFee = s.Cfg.StoneFee(giveStones), s.Cfg.StoneFee(takeStones)
			ov.ValueToReceiver, ov.ValueToSender = 0, 0
			for _, l := range ov.Give {
				ov.ValueToReceiver += l.Quantity * l.UnitValue
			}
			for _, l := range ov.Take {
				ov.ValueToSender += l.Quantity * l.UnitValue
			}
		}
		out = append(out, ov)
	}
	return out, nil
}

// History is GET /trades/history: answered or lapsed offers, newest first.
func (s *Service) History(ctx context.Context, userID uuid.UUID) ([]OfferView, error) {
	q := db.New(s.Pool)
	ch, err := s.me(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	now := s.Now().UTC()
	rows, err := q.ListTradeHistory(ctx, db.ListTradeHistoryParams{CharacterID: ch.ID, Now: now, Lim: s.Cfg.Offers.HistoryLimit})
	if err != nil {
		return nil, err
	}
	offers := make([]offerRow, len(rows))
	for i, r := range rows {
		offers[i] = offerRow{o: db.TradeOffer{ID: r.ID, Kind: r.Kind, SenderCharacterID: r.SenderCharacterID, ReceiverCharacterID: r.ReceiverCharacterID,
			Status: r.Status, Relation: r.Relation, ValueToReceiver: r.ValueToReceiver, ValueToSender: r.ValueToSender, SenderFee: r.SenderFee,
			ReceiverFee: r.ReceiverFee, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, RespondedAt: r.RespondedAt}, name: r.CounterpartName, stage: r.CounterpartStageID}
	}
	return s.offerViews(ctx, q, ch.ID, offers, now)
}

// Peer is GET /trades/peer/{characterId}.
func (s *Service) Peer(ctx context.Context, userID, target uuid.UUID) (PeerView, error) {
	q := db.New(s.Pool)
	ch, err := s.me(ctx, q, userID)
	if err != nil {
		return PeerView{}, err
	}
	if ch.ID == target {
		return PeerView{}, ErrSelf
	}
	mp, _, err := s.party(ctx, q, ch.ID)
	if err != nil {
		return PeerView{}, err
	}
	tp, tparty, err := s.party(ctx, q, target)
	if err != nil {
		return PeerView{}, err
	}
	rel, err := s.relation(ctx, q, mp, tp)
	if err != nil {
		return PeerView{}, err
	}
	return PeerView{PartyView: PartyView{CharacterID: tp.ID.String(), Name: tp.Name, StageID: tp.StageID}, Relation: rel,
		ReceiveCap: s.Cfg.ReceiveCap(rel), Blockers: s.Cfg.ReceiveBlockers(tparty, s.Now().UTC())}, nil
}

// ───── mutations ─────

// Create sends a gift (take empty) or a swap offer to receiver. Under the sender's trade_days row lock (serializes the
// sender's creations): idempotency → gates (both sides, for what the plan makes them do) → static caps and today's
// offer count → soft holdings check (re-verified at accept) → open-offer limit → insert offer + lines + log
// (+ net_flag when the two accounts shared a network recently; never blocks).
func (s *Service) Create(ctx context.Context, userID, receiver uuid.UUID, give, take []Line, idemKey string) (View, error) {
	plan, err := s.plan(give, take)
	if err != nil {
		return View{}, err
	}
	q0 := db.New(s.Pool)
	me, err := s.me(ctx, q0, userID)
	if err != nil {
		return View{}, err
	}
	if me.ID == receiver {
		return View{}, ErrSelf
	}
	rrow, err := q0.GetTradeParty(ctx, receiver)
	if errors.Is(err, pgx.ErrNoRows) {
		return View{}, ErrTargetNotFound
	} else if err != nil {
		return View{}, err
	}
	shared, err := s.Net.SharedRecently(ctx, userID, rrow.UserID) // outside the tx (read-only signal)
	if err != nil {
		return View{}, fmt.Errorf("net signal: %w", err)
	}
	var offerID uuid.UUID
	replay := false
	err = postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		now := s.Now().UTC()
		day := gameday.PG(now)
		if err := q.UpsertTradeDay(ctx, db.UpsertTradeDayParams{CharacterID: me.ID, GameDay: day}); err != nil {
			return err
		}
		d, err := q.GetTradeDayForUpdate(ctx, db.GetTradeDayForUpdateParams{CharacterID: me.ID, GameDay: day})
		if err != nil {
			return fmt.Errorf("lock trade day: %w", err)
		}
		if ok, err := sameOffer(ctx, q, me.ID, receiver, plan, idemKey); err != nil {
			return err
		} else if ok {
			replay = true
			return nil
		}
		mrow, mparty, err := s.party(ctx, q, me.ID)
		if err != nil {
			return err
		}
		rrow, rparty, err := s.party(ctx, q, receiver)
		if err != nil {
			return err
		}
		if err := s.gates(plan, mparty, rparty, true, now); err != nil {
			return err
		}
		rel, err := s.relation(ctx, q, mrow, rrow)
		if err != nil {
			return err
		}
		if err := s.Cfg.CheckCreate(plan, rel, dayOf(d)); err != nil {
			return relativeLimit(err, true)
		}
		if lack, err := holds(ctx, q, me.ID, plan.Give, s.Cfg.StoneFee(plan.StonesGiven())); err != nil {
			return err
		} else if lack != "" {
			return &InsufficientError{Side: "self", AssetID: lack}
		}
		if n, err := q.CountOpenTradeOffers(ctx, db.CountOpenTradeOffersParams{CharacterID: me.ID, Now: now}); err != nil {
			return err
		} else if n >= s.Cfg.Offers.MaxOpenOutgoing {
			return ErrTooManyOpen
		}
		o, err := q.InsertTradeOffer(ctx, db.InsertTradeOfferParams{Kind: plan.Kind, SenderCharacterID: me.ID, ReceiverCharacterID: receiver,
			Relation: rel, NetFlag: shared, ValueToReceiver: plan.ValueToReceiver(), ValueToSender: plan.ValueToSender(),
			IdempotencyKey: idemKey, GameDay: day, CreatedAt: now, ExpiresAt: now.Add(s.Cfg.Expiry())})
		if err != nil {
			return err
		}
		offerID = o.ID
		for _, sl := range []struct {
			name  string
			lines []PricedLine
		}{{"give", plan.Give}, {"take", plan.Take}} {
			for _, l := range sl.lines {
				if err := q.InsertTradeLine(ctx, db.InsertTradeLineParams{OfferID: o.ID, Side: sl.name, AssetKind: l.Kind, AssetID: l.AssetID,
					Quantity: l.Quantity}); err != nil {
					return err
				}
			}
		}
		if err := q.BumpTradeOffersCreated(ctx, db.BumpTradeOffersCreatedParams{CharacterID: me.ID, GameDay: day}); err != nil {
			return err
		}
		if err := event(ctx, q, o.ID, "created", me.ID, map[string]any{"kind": plan.Kind, "relation": rel,
			"valueToReceiver": plan.ValueToReceiver(), "valueToSender": plan.ValueToSender()}, now); err != nil {
			return err
		}
		if shared {
			return event(ctx, q, o.ID, "net_flag", me.ID, map[string]any{"reason": "shared_network"}, now)
		}
		return nil
	})
	if pe := (*pgconn.PgError)(nil); errors.As(err, &pe) && pe.Code == "23505" && pe.TableName == "trade_offers" {
		// Same key raced across the game-day boundary (two trade_days rows locked): the other insert won — replay it.
		replay, err = sameOffer(ctx, db.New(s.Pool), me.ID, receiver, plan, idemKey)
		if err == nil && !replay {
			err = ErrKeyReused
		}
	}
	if err != nil {
		return View{}, err
	}
	if shared && !replay && s.Log != nil {
		s.Log.WarnContext(ctx, "trade offer between accounts on the same network (admin review)", "offerId", offerID,
			"sender", me.ID, "receiver", receiver)
	}
	v, err := s.view(ctx, db.New(s.Pool), me.ID)
	v.Replayed = replay
	return v, err
}

// Accept completes an incoming offer atomically; ack = the receiver confirmed "không hoàn tác". One transaction, lock
// order: offer → both trade_days (character_id order) → both spirit-stone accounts (character_id order) → both
// parties' inventory rows (character_id order, item_id order). Everything is re-verified here: trade.yaml + items.yaml
// (a catalogue edit since creation applies), gates, relation, daily caps, balances. Idempotent by key.
func (s *Service) Accept(ctx context.Context, userID, offerID uuid.UUID, ack bool, idemKey string) (View, error) {
	if !ack {
		return View{}, ErrNotAcknowledged
	}
	q0 := db.New(s.Pool)
	me, err := s.me(ctx, q0, userID)
	if err != nil {
		return View{}, err
	}
	expired, replay := false, false
	err = postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		off, err := q.GetTradeOfferForUpdate(ctx, offerID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && off.ReceiverCharacterID != me.ID) {
			return ErrOfferGone // someone else's offer looks like a missing one
		} else if err != nil {
			return err
		}
		if off.Status == "accepted" && off.RespondKey.Valid && off.RespondKey.String == idemKey {
			replay = true
			return nil
		}
		if off.Status != "pending" {
			return ErrOfferGone
		}
		now := s.Now().UTC()
		if !now.Before(off.ExpiresAt) {
			expired = true
			return s.expireLocked(ctx, q, off, now)
		}
		rows, err := q.ListTradeLines(ctx, []uuid.UUID{off.ID})
		if err != nil {
			return err
		}
		plan, err := s.plan(linesOf(rows, "give"), linesOf(rows, "take"))
		if err != nil {
			return err
		}
		srow, sparty, err := s.party(ctx, q, off.SenderCharacterID)
		if err != nil {
			return err
		}
		rrow, rparty, err := s.party(ctx, q, me.ID)
		if err != nil {
			return err
		}
		if err := s.gates(plan, sparty, rparty, false, now); err != nil {
			return err
		}
		rel, err := s.relation(ctx, q, srow, rrow)
		if err != nil {
			return err
		}
		ids := []uuid.UUID{off.SenderCharacterID, me.ID}
		slices.SortFunc(ids, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
		day := gameday.PG(now)
		days := map[uuid.UUID]db.TradeDay{}
		for _, id := range ids {
			if err := q.UpsertTradeDay(ctx, db.UpsertTradeDayParams{CharacterID: id, GameDay: day}); err != nil {
				return err
			}
			d, err := q.GetTradeDayForUpdate(ctx, db.GetTradeDayForUpdateParams{CharacterID: id, GameDay: day})
			if err != nil {
				return fmt.Errorf("lock trade day: %w", err)
			}
			days[id] = d
		}
		if err := s.Cfg.CheckAccept(plan, rel, dayOf(days[off.SenderCharacterID]), dayOf(days[me.ID])); err != nil {
			return relativeLimit(err, false)
		}
		sFee, rFee := s.Cfg.StoneFee(plan.StonesGiven()), s.Cfg.StoneFee(plan.StonesTaken())
		if plan.StonesGiven()+plan.StonesTaken() > 0 {
			for _, id := range ids {
				if err := q.EnsureCurrencyAccount(ctx, db.EnsureCurrencyAccountParams{CharacterID: id, Currency: SpiritStone}); err != nil {
					return err
				}
				if _, err := q.GetCurrencyAccountForUpdate(ctx, db.GetCurrencyAccountForUpdateParams{CharacterID: id, Currency: SpiritStone}); err != nil {
					return fmt.Errorf("lock currency: %w", err)
				}
			}
		}
		if itemIDs := plan.ItemIDs(); len(itemIDs) > 0 {
			slices.Sort(itemIDs)
			for _, id := range ids {
				if _, err := q.GetInventoryItemsForUpdate(ctx, db.GetInventoryItemsForUpdateParams{CharacterID: id, ItemIds: itemIDs}); err != nil {
					return fmt.Errorf("lock inventory: %w", err)
				}
			}
		}
		if err := move(ctx, q, off.ID, off.SenderCharacterID, me.ID, plan.Give, sFee, "other"); err != nil {
			return err
		}
		if err := move(ctx, q, off.ID, me.ID, off.SenderCharacterID, plan.Take, rFee, "self"); err != nil {
			return err
		}
		if err := q.AddTradeDayCompleted(ctx, db.AddTradeDayCompletedParams{CharacterID: off.SenderCharacterID, GameDay: day,
			Received: plan.ValueToSender(), Sent: plan.ValueToReceiver()}); err != nil {
			return err
		}
		if err := q.AddTradeDayCompleted(ctx, db.AddTradeDayCompletedParams{CharacterID: me.ID, GameDay: day,
			Received: plan.ValueToReceiver(), Sent: plan.ValueToSender()}); err != nil {
			return err
		}
		n, err := q.AcceptTradeOffer(ctx, db.AcceptTradeOfferParams{ID: off.ID, RespondedAt: &now, RespondKey: pgtype.Text{String: idemKey, Valid: true},
			AcceptedDay: day, Relation: rel, ValueToReceiver: plan.ValueToReceiver(), ValueToSender: plan.ValueToSender(),
			SenderFee: sFee, ReceiverFee: rFee})
		if err != nil {
			return err
		}
		if n != 1 {
			return ErrOfferGone
		}
		return event(ctx, q, off.ID, "accepted", me.ID, map[string]any{"relation": rel, "senderFee": sFee, "receiverFee": rFee,
			"valueToReceiver": plan.ValueToReceiver(), "valueToSender": plan.ValueToSender()}, now)
	})
	if err != nil {
		return View{}, err
	}
	if expired {
		return View{}, ErrOfferGone
	}
	v, err := s.view(ctx, db.New(s.Pool), me.ID)
	v.Replayed = replay
	return v, err
}

// move transfers lines from → to through the ledgers and burns the giver's stone fee. Rows are locked by the caller.
// fromSide is the giver relative to the caller (for InsufficientError).
func move(ctx context.Context, q *db.Queries, offer, from, to uuid.UUID, lines []PricedLine, fee int64, fromSide string) error {
	key := func(suffix string) string { return "trade:" + offer.String() + ":" + suffix }
	ref := func(reason, suffix string) economy.Ref {
		return economy.Ref{Reason: reason, Type: "trade", ID: offer.String(), IdemKey: key(suffix)}
	}
	for _, l := range lines {
		var err error
		if l.Kind == KindCurrency {
			if _, err = economy.Credit(ctx, q, from, SpiritStone, -l.Quantity, ref(ReasonSend, "out")); err == nil {
				_, err = economy.Credit(ctx, q, to, SpiritStone, l.Quantity, ref(ReasonReceive, "in"))
			}
		} else {
			if _, err = economy.ConsumeItem(ctx, q, from, l.AssetID, l.Quantity, ref(ReasonSend, "out")); err == nil {
				_, err = economy.GrantItem(ctx, q, to, l.AssetID, l.Quantity, ref(ReasonReceive, "in"))
			}
		}
		if errors.Is(err, economy.ErrInsufficient) {
			return &InsufficientError{Side: fromSide, AssetID: l.AssetID}
		} else if err != nil {
			return fmt.Errorf("move %s: %w", l.AssetID, err)
		}
	}
	if fee > 0 {
		if _, err := economy.Credit(ctx, q, from, SpiritStone, -fee, ref(ReasonFee, "fee")); errors.Is(err, economy.ErrInsufficient) {
			return &InsufficientError{Side: fromSide, AssetID: SpiritStone}
		} else if err != nil {
			return fmt.Errorf("trade fee: %w", err)
		}
	}
	return nil
}

func (s *Service) expireLocked(ctx context.Context, q *db.Queries, off db.TradeOffer, now time.Time) error {
	exp := off.ExpiresAt
	if _, err := q.RespondTradeOffer(ctx, db.RespondTradeOfferParams{ID: off.ID, Status: "expired", RespondedAt: &exp}); err != nil {
		return err
	}
	return event(ctx, q, off.ID, "expired", uuid.Nil, nil, now)
}

// Decline (receiver) and Cancel (sender) close a pending offer. Idempotent by key.
func (s *Service) Decline(ctx context.Context, userID, offerID uuid.UUID, idemKey string) (View, error) {
	return s.close(ctx, userID, offerID, idemKey, "declined")
}

func (s *Service) Cancel(ctx context.Context, userID, offerID uuid.UUID, idemKey string) (View, error) {
	return s.close(ctx, userID, offerID, idemKey, "cancelled")
}

func (s *Service) close(ctx context.Context, userID, offerID uuid.UUID, idemKey, status string) (View, error) {
	q0 := db.New(s.Pool)
	me, err := s.me(ctx, q0, userID)
	if err != nil {
		return View{}, err
	}
	expired := false
	err = postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		off, err := q.GetTradeOfferForUpdate(ctx, offerID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOfferGone
		} else if err != nil {
			return err
		}
		owner := off.ReceiverCharacterID
		if status == "cancelled" {
			owner = off.SenderCharacterID
		}
		if owner != me.ID {
			return ErrOfferGone
		}
		if off.Status == status && off.RespondKey.Valid && off.RespondKey.String == idemKey {
			return nil // replay
		}
		if off.Status != "pending" {
			return ErrOfferGone
		}
		now := s.Now().UTC()
		if !now.Before(off.ExpiresAt) {
			expired = true
			return s.expireLocked(ctx, q, off, now)
		}
		if n, err := q.RespondTradeOffer(ctx, db.RespondTradeOfferParams{ID: off.ID, Status: status, RespondedAt: &now,
			RespondKey: pgtype.Text{String: idemKey, Valid: true}}); err != nil {
			return err
		} else if n != 1 {
			return ErrOfferGone
		}
		return event(ctx, q, off.ID, status, me.ID, nil, now)
	})
	if err != nil {
		return View{}, err
	}
	if expired {
		return View{}, ErrOfferGone
	}
	return s.view(ctx, db.New(s.Pool), me.ID)
}

// ExpireDue marks at most limit lapsed pending offers expired (worker, bounded; SKIP LOCKED leaves offers being
// answered alone). Returns how many.
func (s *Service) ExpireDue(ctx context.Context, limit int) (int, error) {
	n := 0
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		now := s.Now().UTC()
		ids, err := q.ExpireTradeOffers(ctx, db.ExpireTradeOffersParams{Now: now, Lim: int32(limit)})
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := event(ctx, q, id, "expired", uuid.Nil, nil, now); err != nil {
				return err
			}
		}
		n = len(ids)
		return nil
	})
	return n, err
}

// ───── admin ─────

// Receiver is a top net receiver over the window.
type Receiver struct {
	CharacterID     uuid.UUID `json:"characterId"`
	Name            string    `json:"name"`
	Received        int64     `json:"received"`
	Sent            int64     `json:"sent"`
	Net             int64     `json:"net"`
	DistinctSenders int32     `json:"distinctSenders"`
	TradesIn        int32     `json:"tradesIn"`
}

// Pair is a lopsided pair (values a → b and b → a).
type Pair struct {
	A      uuid.UUID `json:"a"`
	AName  string    `json:"aName"`
	B      uuid.UUID `json:"b"`
	BName  string    `json:"bName"`
	AToB   int64     `json:"aToB"`
	BToA   int64     `json:"bToA"`
	Trades int32     `json:"trades"`
}

// Funnel is a receiver fed by many distinct senders.
type Funnel struct {
	CharacterID     uuid.UUID `json:"characterId"`
	Name            string    `json:"name"`
	DistinctSenders int32     `json:"distinctSenders"`
	Received        int64     `json:"received"`
	Trades          int32     `json:"trades"`
}

// Flag is an offer between accounts that shared a network.
type Flag struct {
	OfferID         uuid.UUID `json:"offerId"`
	Kind            string    `json:"kind"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"createdAt"`
	SenderID        uuid.UUID `json:"senderId"`
	SenderName      string    `json:"senderName"`
	ReceiverID      uuid.UUID `json:"receiverId"`
	ReceiverName    string    `json:"receiverName"`
	ValueToReceiver int64     `json:"valueToReceiver"`
	ValueToSender   int64     `json:"valueToSender"`
}

// Flows is GET /admin/trades/flows: abnormal one-way flows over the window (warnings only).
type Flows struct {
	Days             int        `json:"days"`
	Since            time.Time  `json:"since"`
	TopNetReceivers  []Receiver `json:"topNetReceivers"`
	LopsidedPairs    []Pair     `json:"lopsidedPairs"`
	Funnels          []Funnel   `json:"funnels"`
	NetFlags         []Flag     `json:"netFlags"`
	PairMinValue     int64      `json:"pairMinValue"`
	LopsidedMinBP    int64      `json:"lopsidedMinBp"`
	FunnelMinSenders int32      `json:"funnelMinSenders"`
}

// Flows builds the admin report (days ≤ 0 → trade.yaml alerts.window_days).
func (s *Service) Flows(ctx context.Context, days int) (Flows, error) {
	a := s.Cfg.Alerts
	if days <= 0 {
		days = a.WindowDays
	}
	days = min(days, 90)
	since := s.Now().UTC().AddDate(0, 0, -days)
	q := db.New(s.Pool)
	f := Flows{Days: days, Since: since, TopNetReceivers: []Receiver{}, LopsidedPairs: []Pair{}, Funnels: []Funnel{}, NetFlags: []Flag{},
		PairMinValue: a.PairMinValue, LopsidedMinBP: a.LopsidedMinBP, FunnelMinSenders: a.FunnelMinSenders}
	recv, err := q.TradeTopNetReceivers(ctx, db.TradeTopNetReceiversParams{Since: &since, Lim: a.ListLimit})
	if err != nil {
		return f, err
	}
	for _, r := range recv {
		f.TopNetReceivers = append(f.TopNetReceivers, Receiver(r))
	}
	pairs, err := q.TradeLopsidedPairs(ctx, db.TradeLopsidedPairsParams{Since: &since, MinValue: a.PairMinValue, MinBp: a.LopsidedMinBP, Lim: a.ListLimit})
	if err != nil {
		return f, err
	}
	for _, p := range pairs {
		f.LopsidedPairs = append(f.LopsidedPairs, Pair(p))
	}
	fun, err := q.TradeFunnels(ctx, db.TradeFunnelsParams{Since: &since, MinSenders: a.FunnelMinSenders, Lim: a.ListLimit})
	if err != nil {
		return f, err
	}
	for _, r := range fun {
		f.Funnels = append(f.Funnels, Funnel(r))
	}
	flags, err := q.ListTradeFlags(ctx, db.ListTradeFlagsParams{Since: since, Lim: a.ListLimit})
	if err != nil {
		return f, err
	}
	for _, r := range flags {
		f.NetFlags = append(f.NetFlags, Flag{OfferID: r.ID, Kind: r.Kind, Status: r.Status, CreatedAt: r.CreatedAt, SenderID: r.SenderCharacterID,
			SenderName: r.SenderName, ReceiverID: r.ReceiverCharacterID, ReceiverName: r.ReceiverName, ValueToReceiver: r.ValueToReceiver,
			ValueToSender: r.ValueToSender})
	}
	return f, nil
}
