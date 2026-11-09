package ops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/platform/postgres"
)

// Beta reset scope (lo-trinh-checklist Phase 4 "Reset dữ liệu beta"; data-model §13 / Q21: truncate characters and
// Tiên Duyên, keep accounts). Every table of the public schema must be classified here: ResetBeta refuses to run
// while an unclassified table exists, so a new migration forces an explicit keep/reset decision.

// KeptTables survive a beta reset: accounts and sessions, the admin audit trail, user-level moderation, the salted
// network hashes (expire on their own after 30 days) and the migration state.
func KeptTables() []string {
	return []string{"schema_migrations", "users", "user_identities", "refresh_tokens", "admin_audit_logs", "chat_mutes",
		"net_signals"}
}

// DuyenTables hold Tiên Duyên (điểm duyên). Reset by default (Q21); kept with ResetOptions.KeepDuyen.
func DuyenTables() []string {
	return []string{"vip_accounts", "vip_point_ledger", "vip_daily_gift_claims"}
}

// PaymentTables are financial records: always kept, except sandbox-only test data with ResetOptions.WipePurchases.
func PaymentTables() []string {
	return []string{"payment_orders", "payment_events", "payment_sandbox_charges"}
}

// ResetTables are truncated: every character-owned table (CASCADE from characters would reach them anyway), user-level
// game state, the outbox and the world clock (world_state gets its default row back: started_at NULL = pre-launch).
func ResetTables() []string {
	return []string{
		// character data
		"characters", "character_progressions", "cultivation_settlements", "breakthrough_attempts", "progression_events",
		"currency_accounts", "currency_ledger", "inventory_items", "inventory_transactions", "daily_task_progress",
		"daily_contribution", "cultivation_credits", "item_daily_uses", "lecture_insights", "chat_messages", "reports",
		"character_faction", "faction_history", "shop_purchases", "shop_daily_counts", "ritual_streaks", "daily_fortunes",
		"character_buffs", "trial_runs", "character_cosmetics", "character_appearance", "cosmetic_equips",
		"faction_donations", "faction_donation_days", "faction_donors", "secret_realm_runs", "secret_realm_days",
		"character_artifacts", "protector_invites", "protector_assists", "protector_days",
		"dao_lu_proposals", "dao_lu_bonds", "dao_lu_members", "dao_lu_events", "dao_lu_song_tu_claims",
		"trade_offers", "trade_lines", "trade_days", "trade_events", "tutorial_progress", "tutorial_prefs",
		// user-level game state + pipeline
		"late_join_grants", "vip_entitlements", "outbox_events",
		// world
		"world_state", "world_daily_totals", "world_milestones_reached", "world_effects", "world_events",
		"world_milestone_overrides", "faction_treasuries", "realm_firsts", "world_announcements",
	}
}

// ResetConfirmPhrase is what the operator must type to reset database db.
func ResetConfirmPhrase(db string) string { return "reset " + db }

// ResetOptions control ResetBeta.
type ResetOptions struct {
	Admin          uuid.UUID // must be an existing admin user (audit row)
	KeepDuyen      bool      // keep vip_* tables (paid and earned điểm duyên)
	WipePurchases  bool      // with Duyên wiped: also wipe purchase rows and the payment tables — sandbox orders only
	Reason         string    // stored in the audit row
	ConfirmPhrase  string    // must equal ResetConfirmPhrase(Summary.Database)
	ExpectedTables []string  // optional override of the classification (tests); nil = Kept+Duyen+Reset
}

// Summary is what an operator sees before confirming.
type Summary struct {
	Database     string           `json:"database"`
	Counts       map[string]int64 `json:"counts"` // rows per table that will be truncated
	Kept         []string         `json:"kept"`
	Unclassified []string         `json:"unclassified,omitempty"`
	Purchases    int64            `json:"purchases"`    // vip_point_ledger rows with reason 'purchase' or 'refund'
	RealPayments int64            `json:"realPayments"` // payment_orders of a non-sandbox provider (never wiped)
	WorldStarted bool             `json:"worldStarted"`
}

// Total is the number of rows that will be deleted.
func (s Summary) Total() int64 {
	var n int64
	for _, c := range s.Counts {
		n += c
	}
	return n
}

var (
	ErrUnclassified = errors.New("ops: unclassified tables in the public schema (classify them in ops.ResetTables/KeptTables)")
	ErrConfirm      = errors.New("ops: confirmation phrase does not match")
	ErrNotAdmin     = errors.New("ops: --admin must be an existing user with role admin")
	ErrPurchases    = errors.New("ops: vip_point_ledger holds top-up rows; pass --keep-duyen (or --wipe-purchases for sandbox-only data)")
	ErrRealPayments = errors.New("ops: real-provider payment orders exist; Tiên Duyên must be kept (--keep-duyen)")
)

func (o ResetOptions) wiped() []string {
	t := ResetTables()
	if !o.KeepDuyen {
		t = append(t, DuyenTables()...)
		if o.WipePurchases {
			t = append(t, PaymentTables()...)
		}
	}
	return t
}

func (o ResetOptions) kept() []string {
	t := KeptTables()
	if o.KeepDuyen {
		t = append(t, DuyenTables()...)
	}
	if o.KeepDuyen || !o.WipePurchases {
		t = append(t, PaymentTables()...)
	}
	return t
}

// Plan reads the reset summary without changing anything.
func Plan(ctx context.Context, pool *pgxpool.Pool, o ResetOptions) (Summary, error) {
	var s Summary
	if err := pool.QueryRow(ctx, "SELECT current_database()").Scan(&s.Database); err != nil {
		return s, err
	}
	rows, err := pool.Query(ctx, `SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE' ORDER BY table_name`)
	if err != nil {
		return s, err
	}
	present, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return s, err
	}
	known := slices.Concat(KeptTables(), DuyenTables(), PaymentTables(), ResetTables())
	if o.ExpectedTables != nil {
		known = o.ExpectedTables
	}
	wiped, keptSet := o.wiped(), o.kept()
	s.Counts = map[string]int64{}
	for _, t := range present {
		switch {
		case !slices.Contains(known, t):
			s.Unclassified = append(s.Unclassified, t)
		case slices.Contains(wiped, t):
			var n int64
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{t}.Sanitize()).Scan(&n); err != nil {
				return s, fmt.Errorf("count %s: %w", t, err)
			}
			s.Counts[t] = n
		case slices.Contains(keptSet, t):
			s.Kept = append(s.Kept, t)
		}
	}
	if slices.Contains(present, "vip_point_ledger") {
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM vip_point_ledger WHERE reason IN ('purchase', 'refund')").Scan(&s.Purchases); err != nil {
			return s, err
		}
	}
	if slices.Contains(present, "payment_orders") {
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM payment_orders WHERE provider <> 'sandbox'").Scan(&s.RealPayments); err != nil {
			return s, err
		}
	}
	if err := pool.QueryRow(ctx, "SELECT COALESCE((SELECT started_at IS NOT NULL FROM world_state WHERE id = 1), false)").
		Scan(&s.WorldStarted); err != nil {
		return s, err
	}
	return s, nil
}

// ResetBeta truncates the beta data in one transaction, restores the pre-launch world_state row and writes an
// admin_audit_logs row (action ops.reset_beta, before = the summary). Redis is NOT touched here (caller rebuilds it
// after the commit). It re-plans inside the call so the summary audited is the one actually applied.
func ResetBeta(ctx context.Context, pool *pgxpool.Pool, o ResetOptions) (Summary, error) {
	s, err := Plan(ctx, pool, o)
	if err != nil {
		return s, err
	}
	if len(s.Unclassified) > 0 {
		return s, fmt.Errorf("%w: %s", ErrUnclassified, strings.Join(s.Unclassified, ", "))
	}
	if o.ConfirmPhrase != ResetConfirmPhrase(s.Database) {
		return s, ErrConfirm
	}
	if !o.KeepDuyen && s.RealPayments > 0 {
		return s, ErrRealPayments
	}
	if !o.KeepDuyen && s.Purchases > 0 && !o.WipePurchases {
		return s, ErrPurchases
	}
	var role string
	if err := pool.QueryRow(ctx, "SELECT role FROM users WHERE id = $1", o.Admin).Scan(&role); err != nil || role != "admin" {
		return s, ErrNotAdmin
	}
	// Tables referenced by a KEPT table (vip_point_ledger ← payment_orders) cannot be TRUNCATEd: they are emptied with
	// DELETE, children first, which also fails loudly if a kept row still points at them.
	var tables, deletes []string
	keptRefs := !o.KeepDuyen && !o.WipePurchases && s.hasAny(PaymentTables())
	for t := range s.Counts {
		if keptRefs && slices.Contains(DuyenTables(), t) {
			continue
		}
		tables = append(tables, pgx.Identifier{t}.Sanitize())
	}
	slices.Sort(tables)
	if keptRefs {
		for _, t := range []string{"vip_daily_gift_claims", "vip_point_ledger", "vip_accounts"} {
			if _, ok := s.Counts[t]; ok {
				deletes = append(deletes, pgx.Identifier{t}.Sanitize())
			}
		}
	}
	before, _ := json.Marshal(s)
	err = postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
		if len(tables) > 0 {
			// No CASCADE: the list is complete by construction; a missing FK target would fail loudly instead of
			// silently wiping a kept table.
			if _, err := tx.Exec(ctx, "TRUNCATE "+strings.Join(tables, ", ")+" RESTART IDENTITY"); err != nil {
				return fmt.Errorf("truncate: %w", err)
			}
		}
		for _, t := range deletes {
			if _, err := tx.Exec(ctx, "DELETE FROM "+t); err != nil {
				return fmt.Errorf("delete %s: %w", t, err)
			}
		}
		if _, err := tx.Exec(ctx, "INSERT INTO world_state (id) VALUES (1) ON CONFLICT (id) DO NOTHING"); err != nil {
			return fmt.Errorf("world_state: %w", err)
		}
		_, err := tx.Exec(ctx, `INSERT INTO admin_audit_logs (admin_user_id, action, target_type, target_id, before, after, reason)
			VALUES ($1, 'ops.reset_beta', 'database', $2, $3, '{}'::jsonb, NULLIF($4, ''))`, o.Admin, s.Database, before, o.Reason)
		return err
	})
	return s, err
}

func (s Summary) hasAny(tables []string) bool {
	for _, t := range tables {
		if slices.Contains(s.Kept, t) {
			return true
		}
	}
	return false
}

// ResolveAdmin finds an admin by UUID or e-mail.
func ResolveAdmin(ctx context.Context, pool *pgxpool.Pool, ref string) (uuid.UUID, error) {
	var id uuid.UUID
	var role string
	err := pool.QueryRow(ctx, "SELECT id, role FROM users WHERE id::text = $1 OR lower(email) = lower($1) LIMIT 1", ref).Scan(&id, &role)
	if err != nil || role != "admin" {
		return uuid.Nil, ErrNotAdmin
	}
	return id, nil
}
