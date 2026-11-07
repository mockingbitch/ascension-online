// Package report handles player reports (offensive name, harassment) and their admin resolution.
package report

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/character"
	"tiendao/game/internal/db"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/moderation"
	"tiendao/game/internal/platform/postgres"
)

// Report reasons and statuses (mirrored by CHECK constraints in migration 000006).
const (
	ReasonOffensiveName = "offensive_name"
	ReasonHarassment    = "harassment"
	ReasonTradeAbuse    = "trade_abuse" // about a trade offer (Phase 5.2): only via SubmitTrade, by one party about the other

	StatusOpen      = "open"
	StatusDismissed = "dismissed"
	StatusActioned  = "actioned"
)

var (
	ErrInvalidReport     = errors.New("report: invalid report")
	ErrUnknownReporter   = errors.New("report: reporter account not found")
	ErrTargetNotFound    = errors.New("report: reported character not found")
	ErrSelfReport        = errors.New("report: cannot report your own character")
	ErrDailyLimit        = errors.New("report: daily report limit reached")
	ErrKeyReuse          = errors.New("report: idempotency key reused for a different report")
	ErrReportNotFound    = errors.New("report: report not found")
	ErrAlreadyResolved   = errors.New("report: report already resolved")
	ErrInvalidResolution = errors.New("report: invalid resolution")
)

// Report is what the reporter sees.
type Report struct {
	ID                uuid.UUID  `json:"id"`
	TargetCharacterID uuid.UUID  `json:"targetCharacterId"`
	TargetName        string     `json:"targetName"`
	Reason            string     `json:"reason"`
	Note              string     `json:"note"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"createdAt"`
	ChatMessageID     *int64     `json:"chatMessageId"`
	TradeOfferID      *uuid.UUID `json:"tradeOfferId"`
}

// AdminReport is a queue entry for moderators.
type AdminReport struct {
	Report
	ReporterUserID    uuid.UUID  `json:"reporterUserId"`
	ReporterName      string     `json:"reporterName"`
	TargetUserID      uuid.UUID  `json:"targetUserId"`
	TargetCurrentName string     `json:"targetCurrentName"`
	OpenAgainstTarget int64      `json:"openAgainstTarget"`
	ResolutionNote    string     `json:"resolutionNote"`
	ResolvedAt        *time.Time `json:"resolvedAt"`
	Evidence          string     `json:"evidence"` // chat text at report time (original, before masking)
}

// Resolution is the result of resolving one report.
type Resolution struct {
	Report         Report      `json:"report"`
	ResolutionNote string      `json:"resolutionNote"`
	RenamedFrom    string      `json:"renamedFrom,omitempty"`
	RenamedTo      string      `json:"renamedTo,omitempty"`
	AlsoResolved   []uuid.UUID `json:"alsoResolved"`
}

// Service owns report transactions.
type Service struct {
	Pool *pgxpool.Pool
	Cat  *moderation.Catalogue
	Now  func() time.Time
}

func toReport(r db.Report) Report {
	return Report{ID: r.ID, TargetCharacterID: r.TargetCharacterID, TargetName: r.TargetName, Reason: r.Reason,
		Note: r.Note, Status: r.Status, CreatedAt: r.CreatedAt, ChatMessageID: int8Ptr(r.ChatMessageID), TradeOfferID: uuidPtr(r.TradeOfferID)}
}

func uuidPtr(v pgtype.UUID) *uuid.UUID {
	if !v.Valid {
		return nil
	}
	id := uuid.UUID(v.Bytes)
	return &id
}

func int8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func validReason(r string) bool {
	return r == ReasonOffensiveName || r == ReasonHarassment || r == ReasonTradeAbuse
}

// Submit files a report. Idempotent per (reporter, key); a second OPEN report on the same target+reason returns
// the existing one without counting toward the daily limit. chatMessageID (0 = none) must be a message sent by
// the target; its original text is snapshotted as evidence.
func (s *Service) Submit(ctx context.Context, reporter, target uuid.UUID, reason, note, key string, chatMessageID int64) (Report, error) {
	return s.submit(ctx, reporter, target, reason, note, key, chatMessageID, uuid.Nil)
}

// SubmitTrade reports the other party of a trade offer the reporter took part in (reason trade_abuse; the offer is
// attached as evidence). Same idempotency, dedupe and daily limit as Submit.
func (s *Service) SubmitTrade(ctx context.Context, reporter, offerID uuid.UUID, note, key string) (Report, error) {
	o, err := db.New(s.Pool).GetTradeOfferBrief(ctx, offerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Report{}, ErrInvalidReport
	} else if err != nil {
		return Report{}, err
	}
	var target uuid.UUID
	switch reporter {
	case o.SenderUserID:
		target = o.ReceiverCharacterID
	case o.ReceiverUserID:
		target = o.SenderCharacterID
	default:
		return Report{}, ErrInvalidReport // not a party: looks like a missing offer
	}
	return s.submit(ctx, reporter, target, ReasonTradeAbuse, note, key, 0, offerID)
}

func (s *Service) submit(ctx context.Context, reporter, target uuid.UUID, reason, note, key string, chatMessageID int64, tradeOfferID uuid.UUID) (Report, error) {
	note = strings.TrimSpace(note)
	if (tradeOfferID == uuid.Nil) == (reason == ReasonTradeAbuse) {
		return Report{}, ErrInvalidReport
	}
	if !validReason(reason) || utf8.RuneCountInString(note) > s.Cat.Reports.NoteMaxChars || !utf8.ValidString(note) {
		return Report{}, ErrInvalidReport
	}
	now := s.Now().UTC()
	var out Report
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		if _, err := q.LockUser(ctx, reporter); errors.Is(err, pgx.ErrNoRows) {
			return ErrUnknownReporter
		} else if err != nil {
			return err
		}
		if prev, err := q.GetReportByKey(ctx, db.GetReportByKeyParams{ReporterUserID: reporter, IdempotencyKey: key}); err == nil {
			if prev.TargetCharacterID != target || prev.Reason != reason {
				return ErrKeyReuse
			}
			out = toReport(prev)
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		ch, err := q.GetCharacterByID(ctx, target)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTargetNotFound
		} else if err != nil {
			return err
		}
		if ch.UserID == reporter {
			return ErrSelfReport
		}
		var msgID pgtype.Int8
		evidence := ""
		if chatMessageID != 0 {
			m, err := q.GetChatMessage(ctx, chatMessageID)
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && m.CharacterID != target) {
				return ErrInvalidReport
			} else if err != nil {
				return err
			}
			msgID = pgtype.Int8{Int64: m.ID, Valid: true}
			evidence = m.Body
			if m.RawBody.Valid {
				evidence = m.RawBody.String
			}
		}
		var offer pgtype.UUID
		if tradeOfferID != uuid.Nil {
			offer = pgtype.UUID{Bytes: tradeOfferID, Valid: true}
			evidence = "trade offer " + tradeOfferID.String()
		}
		if open, err := q.GetOpenReport(ctx, db.GetOpenReportParams{ReporterUserID: reporter, TargetCharacterID: target, Reason: reason}); err == nil {
			out = toReport(open)
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		n, err := q.CountReportsSince(ctx, db.CountReportsSinceParams{ReporterUserID: reporter, Since: gameday.NextReset(now).Add(-24 * time.Hour)})
		if err != nil {
			return err
		}
		if n >= int64(s.Cat.Reports.DailyLimit) {
			return ErrDailyLimit
		}
		r, err := q.InsertReport(ctx, db.InsertReportParams{ReporterUserID: reporter, TargetCharacterID: target,
			TargetName: ch.Name, Reason: reason, Note: note, IdempotencyKey: key, Now: now, ChatMessageID: msgID, Evidence: evidence,
			TradeOfferID: offer})
		if err != nil {
			return err
		}
		out = toReport(r)
		return nil
	})
	return out, err
}

// List returns reports with the given status, oldest first.
func (s *Service) List(ctx context.Context, status string, limit, offset int32) ([]AdminReport, error) {
	if status != StatusOpen && status != StatusDismissed && status != StatusActioned {
		return nil, ErrInvalidResolution
	}
	rows, err := db.New(s.Pool).ListReports(ctx, db.ListReportsParams{Status: status, Lim: limit, Off: offset})
	if err != nil {
		return nil, err
	}
	out := make([]AdminReport, 0, len(rows))
	for _, r := range rows {
		out = append(out, AdminReport{
			Report: Report{ID: r.ID, TargetCharacterID: r.TargetCharacterID, TargetName: r.TargetName, Reason: r.Reason,
				Note: r.Note, Status: r.Status, CreatedAt: r.CreatedAt, ChatMessageID: int8Ptr(r.ChatMessageID), TradeOfferID: uuidPtr(r.TradeOfferID)},
			ReporterUserID: r.ReporterUserID, ReporterName: r.ReporterName, TargetUserID: r.TargetUserID,
			TargetCurrentName: r.TargetCurrentName, OpenAgainstTarget: r.OpenAgainstTarget,
			ResolutionNote: r.ResolutionNote, ResolvedAt: r.ResolvedAt, Evidence: r.Evidence,
		})
	}
	return out, nil
}

// Resolve closes an open report as dismissed or actioned. With renameTo (actioned only), the reported character is
// renamed in the same transaction and every other open offensive-name report against it is closed too.
// Each change writes an admin_audit_logs row.
func (s *Service) Resolve(ctx context.Context, admin, id uuid.UUID, outcome, note, renameTo string) (Resolution, error) {
	note = strings.TrimSpace(note)
	renameTo = strings.TrimSpace(renameTo)
	if (outcome != StatusDismissed && outcome != StatusActioned) || utf8.RuneCountInString(note) > 1000 ||
		(renameTo != "" && outcome != StatusActioned) {
		return Resolution{}, ErrInvalidResolution
	}
	now := s.Now().UTC()
	adminID := pgtype.UUID{Bytes: admin, Valid: true}
	var out Resolution
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		r, err := q.GetReportForUpdate(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReportNotFound
		} else if err != nil {
			return err
		}
		if r.Status != StatusOpen {
			return ErrAlreadyResolved
		}
		if renameTo != "" {
			from, to, err := character.Rename(ctx, q, r.TargetCharacterID, renameTo, s.Cat)
			if err != nil {
				return err
			}
			out.RenamedFrom, out.RenamedTo = from, to
			if err := audit(ctx, q, admin, "character.rename", "character", r.TargetCharacterID.String(),
				map[string]any{"name": from}, map[string]any{"name": to, "reportId": id}, note); err != nil {
				return err
			}
			ids, err := q.ResolveOpenReportsForTarget(ctx, db.ResolveOpenReportsForTargetParams{
				Note: "Đã đổi tên (xử lý cùng báo cáo " + id.String() + ")", Admin: adminID, Now: &now,
				Target: r.TargetCharacterID, Reason: ReasonOffensiveName, ExceptID: id,
			})
			if err != nil {
				return err
			}
			for _, other := range ids {
				if err := audit(ctx, q, admin, "report.resolve", "report", other.String(),
					map[string]any{"status": StatusOpen}, map[string]any{"status": StatusActioned, "via": id}, note); err != nil {
					return err
				}
			}
			out.AlsoResolved = ids
		}
		upd, err := q.ResolveReport(ctx, db.ResolveReportParams{Status: outcome, Note: note, Admin: adminID, Now: &now, ID: id})
		if err != nil {
			return err
		}
		out.Report, out.ResolutionNote = toReport(upd), upd.ResolutionNote
		return audit(ctx, q, admin, "report.resolve", "report", id.String(),
			map[string]any{"status": StatusOpen}, map[string]any{"status": outcome, "renamedTo": out.RenamedTo}, note)
	})
	if out.AlsoResolved == nil {
		out.AlsoResolved = []uuid.UUID{}
	}
	return out, err
}

func audit(ctx context.Context, q *db.Queries, admin uuid.UUID, action, targetType, targetID string, before, after map[string]any, note string) error {
	b, err := json.Marshal(before)
	if err != nil {
		return err
	}
	a, err := json.Marshal(after)
	if err != nil {
		return err
	}
	return q.InsertAdminAudit(ctx, db.InsertAdminAuditParams{AdminUserID: admin, Action: action, TargetType: targetType,
		TargetID: targetID, Before: b, After: a, Reason: pgtype.Text{String: note, Valid: note != ""}})
}
