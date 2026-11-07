package factionwork

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/db"
	"tiendao/game/internal/economy"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/platform/postgres"
)

// Ledger reason of a donation debit.
const reasonDonation = "faction_donation"

// FactionTotal is one faction's pool.
type FactionTotal struct {
	FactionID    string `json:"factionId"`
	DonatedTotal int64  `json:"donatedTotal"`
}

// Donor is one row of the công đức bảng (display only).
type Donor struct {
	CharacterID uuid.UUID `json:"characterId"`
	Name        string    `json:"name"`
	Total       int64     `json:"total"`
}

// Mine is the caller's faction detail.
type Mine struct {
	FactionID string   `json:"factionId"`
	Progress  Progress `json:"progress"`
	MyTotal   int64    `json:"myTotal"`
	Donors    []Donor  `json:"donors"`
}

// View is GET /factions/works and the state returned by a donation (the client replaces, never merges).
type View struct {
	Settings     Settings       `json:"settings"`
	Works        []Work         `json:"works"`
	Factions     []FactionTotal `json:"factions"` // every faction with a pool (others = 0)
	Balance      int64          `json:"balance"`
	TodayDonated int64          `json:"todayDonated"`
	GameDay      string         `json:"gameDay"`
	ResetsAt     time.Time      `json:"resetsAt"`
	Mine         *Mine          `json:"mine"` // null = not in a faction (cannot donate)
}

// DonateResult is POST /factions/donate.
type DonateResult struct {
	DonationID int64 `json:"donationId"`
	Amount     int64 `json:"amount"`
	Replayed   bool  `json:"replayed"`
	State      View  `json:"state"`
}

// Service owns donations. It deliberately has no dependency on cultivation, vip or leaderboard: see package doc.
type Service struct {
	Pool *pgxpool.Pool
	Cat  *Catalogue
	Now  func() time.Time
}

func character(ctx context.Context, q *db.Queries, userID uuid.UUID) (db.Character, error) {
	ch, err := q.GetCharacterByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ch, ErrNoCharacter
	}
	return ch, err
}

// View returns the works state for the caller (read-only, no locks).
func (s *Service) View(ctx context.Context, userID uuid.UUID) (View, error) {
	q := db.New(s.Pool)
	ch, err := character(ctx, q, userID)
	if err != nil {
		return View{}, err
	}
	return s.view(ctx, q, ch.ID, s.Now())
}

func (s *Service) view(ctx context.Context, q *db.Queries, charID uuid.UUID, now time.Time) (View, error) {
	gd := gameday.PG(now)
	v := View{Settings: s.Cat.Settings, Works: s.Cat.All(), Factions: []FactionTotal{},
		GameDay: gameday.Of(now).Format(time.DateOnly), ResetsAt: gameday.NextReset(now).UTC()}
	var err error
	if v.Balance, err = q.GetCurrencyBalance(ctx, db.GetCurrencyBalanceParams{CharacterID: charID, Currency: Currency}); err != nil {
		return View{}, err
	}
	if v.TodayDonated, err = q.GetFactionDonationDay(ctx, db.GetFactionDonationDayParams{CharacterID: charID, GameDay: gd}); err != nil {
		return View{}, err
	}
	rows, err := q.ListFactionTreasuries(ctx)
	if err != nil {
		return View{}, err
	}
	for _, r := range rows {
		v.Factions = append(v.Factions, FactionTotal{FactionID: r.FactionID, DonatedTotal: r.DonatedTotal})
	}
	cf, err := q.GetCharacterFaction(ctx, charID)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, nil
	} else if err != nil {
		return View{}, err
	}
	total, err := q.GetFactionTreasury(ctx, cf.FactionID)
	if err != nil {
		return View{}, err
	}
	mine := &Mine{FactionID: cf.FactionID, Progress: s.Cat.ProgressOf(total), Donors: []Donor{}}
	if mine.MyTotal, err = q.GetFactionDonorTotal(ctx, db.GetFactionDonorTotalParams{FactionID: cf.FactionID, CharacterID: charID}); err != nil {
		return View{}, err
	}
	donors, err := q.ListFactionDonors(ctx, db.ListFactionDonorsParams{FactionID: cf.FactionID, Lim: s.Cat.Settings.DonorBoardSize})
	if err != nil {
		return View{}, err
	}
	for _, d := range donors {
		mine.Donors = append(mine.Donors, Donor{CharacterID: d.CharacterID, Name: d.Name, Total: d.Total})
	}
	v.Mine = mine
	return v, nil
}

// Donate gives `amount` spirit stones to the caller's current faction in ONE transaction. Lock order (data-model
// §2.3): character_faction (FOR UPDATE: the faction can't change mid-donation) → currency_accounts → idempotency
// (faction_donations key) → validate (amount range, daily cap, balance) → faction_donation_days (conditional upsert)
// → debit with ledger → faction_donors → faction_treasuries + donation row in ONE final statement (the shared hot row
// is locked last and released at the commit right after; anti-cheat M3). The view is read AFTER the commit.
//
// It writes NOTHING that feeds faction merit (daily_contribution), ranking (leaderboard outbox), điểm duyên
// (vip_point_ledger) or tu vi (character_progressions) — the sink buys only visuals (tested).
func (s *Service) Donate(ctx context.Context, userID uuid.UUID, amount int64, idemKey string) (DonateResult, error) {
	var out DonateResult
	var charID uuid.UUID
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		ch, err := character(ctx, q, userID)
		if err != nil {
			return err
		}
		charID = ch.ID
		cf, err := q.GetCharacterFactionForUpdate(ctx, ch.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoFaction
		} else if err != nil {
			return fmt.Errorf("lock faction membership: %w", err)
		}
		if err := q.EnsureCurrencyAccount(ctx, db.EnsureCurrencyAccountParams{CharacterID: ch.ID, Currency: Currency}); err != nil {
			return err
		}
		acc, err := q.GetCurrencyAccountForUpdate(ctx, db.GetCurrencyAccountForUpdateParams{CharacterID: ch.ID, Currency: Currency})
		if err != nil {
			return fmt.Errorf("lock currency: %w", err)
		}
		now := s.Now()
		if prev, err := q.GetFactionDonationByKey(ctx, db.GetFactionDonationByKeyParams{CharacterID: ch.ID, IdempotencyKey: idemKey}); err == nil {
			if prev.Amount != amount {
				return ErrKeyReuse
			}
			out = DonateResult{DonationID: prev.ID, Amount: prev.Amount, Replayed: true}
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		gd := gameday.PG(now)
		today, err := q.GetFactionDonationDay(ctx, db.GetFactionDonationDayParams{CharacterID: ch.ID, GameDay: gd})
		if err != nil {
			return err
		}
		if err := s.Cat.Check(amount, today, acc.Balance); err != nil {
			return err
		}
		if _, err := q.IncrementFactionDonationDay(ctx, db.IncrementFactionDonationDayParams{
			CharacterID: ch.ID, GameDay: gd, Amount: amount, Cap: s.Cat.Settings.DailyCapPerCharacter,
		}); errors.Is(err, pgx.ErrNoRows) {
			return ErrDailyCap
		} else if err != nil {
			return fmt.Errorf("donation day: %w", err)
		}
		ref := economy.Ref{Reason: reasonDonation, Type: "faction", ID: cf.FactionID, IdemKey: "donate:" + idemKey}
		if _, err := economy.Credit(ctx, q, ch.ID, Currency, -amount, ref); errors.Is(err, economy.ErrInsufficient) {
			return ErrInsufficient
		} else if err != nil {
			return fmt.Errorf("debit: %w", err)
		}
		if _, err := q.AddFactionDonor(ctx, db.AddFactionDonorParams{FactionID: cf.FactionID, CharacterID: ch.ID, Amount: amount, Now: now}); err != nil {
			return fmt.Errorf("donor board: %w", err)
		}
		row, err := q.RecordFactionDonation(ctx, db.RecordFactionDonationParams{
			CharacterID: ch.ID, FactionID: cf.FactionID, Amount: amount, GameDay: gd, IdempotencyKey: idemKey, Now: now,
		})
		if err != nil {
			return fmt.Errorf("record donation: %w", err)
		}
		out = DonateResult{DonationID: row.ID, Amount: amount}
		return nil
	})
	if err != nil {
		return DonateResult{}, err
	}
	// Plain reads after the commit: no lock is held while the (bigger) view is assembled.
	out.State, err = s.view(ctx, db.New(s.Pool), charID, s.Now())
	return out, err
}
