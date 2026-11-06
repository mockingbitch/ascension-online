package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/db"
	"tiendao/game/internal/moderation"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/realm"
)

// WS event types (packages/contracts/ws-messages.md).
const (
	EventMessage = "chat.message"
	EventHidden  = "chat.hidden"
)

var (
	ErrUnknownChannel = errors.New("chat: unknown channel")
	ErrNoCharacter    = errors.New("chat: account has no character")
	ErrDuplicate      = errors.New("chat: same message sent too recently")
	ErrKeyReuse       = errors.New("chat: idempotency key reused for a different message")
	ErrNotFound       = errors.New("chat: message not found")
	ErrInvalidMute    = errors.New("chat: invalid mute")
	ErrUnknownUser    = errors.New("chat: user not found")
)

// StageTooLowError: posting needs MinStage (reading is open to everyone).
type StageTooLowError struct{ MinStage string }

func (e *StageTooLowError) Error() string {
	return "chat: stage too low to post (needs " + e.MinStage + ")"
}

// MutedError: the account is muted until Until.
type MutedError struct{ Until time.Time }

func (e *MutedError) Error() string { return "chat: muted until " + e.Until.Format(time.RFC3339) }

// Message is the public shape of a chat message (REST history + WS `chat.message`).
type Message struct {
	ID          int64     `json:"id"`
	Channel     string    `json:"channel"`
	CharacterID uuid.UUID `json:"characterId"`
	Name        string    `json:"name"`
	StageID     string    `json:"stageId"`
	Text        string    `json:"text"`
	SentAt      time.Time `json:"sentAt"`
}

// Publisher fans an event out to every API instance (realtime.Publish in production). Best effort.
type Publisher func(ctx context.Context, eventType, id string, data any) error

// Service owns chat transactions.
type Service struct {
	Pool    *pgxpool.Pool
	Cat     *Catalogue
	Realms  *realm.Catalogue
	Mod     *moderation.Catalogue
	Now     func() time.Time
	Publish Publisher // nil → no live delivery (clients still get history)
}

// CanPost reports whether a character at stageID may post in ch. Stages on another path compare by tier/order.
func CanPost(realms *realm.Catalogue, ch Channel, stageID string) bool {
	s, err := realms.Stage(stageID)
	if err != nil {
		return false
	}
	min, err := realms.Stage(ch.MinStage)
	if err != nil {
		return false
	}
	if min.Path != s.Path {
		if min, err = realms.Convert(min, s.Path); err != nil {
			return false
		}
	}
	return s.Index >= min.Index
}

// Send posts a message. Idempotent per (user, key): a replay returns the stored message without re-broadcasting.
func (s *Service) Send(ctx context.Context, userID uuid.UUID, channel, text, key string) (Message, error) {
	ch, ok := s.Cat.Channel(channel)
	if !ok {
		return Message{}, ErrUnknownChannel
	}
	clean, err := CleanText(text, s.Cat.MaxChars)
	if err != nil {
		return Message{}, err
	}
	body, hits := s.Mod.MaskText(clean, s.Cat.MaskCategories)
	raw := pgtype.Text{String: clean, Valid: hits > 0}
	now := s.Now().UTC()

	var out Message
	replay := false
	err = postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		if _, err := q.LockUser(ctx, userID); errors.Is(err, pgx.ErrNoRows) {
			return ErrUnknownUser
		} else if err != nil {
			return err
		}
		if prev, err := q.GetChatMessageByKey(ctx, db.GetChatMessageByKeyParams{UserID: userID, IdempotencyKey: key}); err == nil {
			if prev.Channel != channel || prev.Body != body {
				return ErrKeyReuse
			}
			out, replay = fromRow(prev), true
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		sender, err := q.GetChatSender(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoCharacter
		} else if err != nil {
			return err
		}
		if !CanPost(s.Realms, ch, sender.StageID) {
			return &StageTooLowError{MinStage: ch.MinStage}
		}
		if until, err := q.GetChatMute(ctx, userID); err == nil && until.After(now) {
			return &MutedError{Until: until}
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if s.Cat.DuplicateWindow > 0 {
			dup, err := q.RecentSameText(ctx, db.RecentSameTextParams{UserID: userID, Since: now.Add(-s.Cat.DuplicateWindow), Body: body})
			if err != nil {
				return err
			}
			if dup {
				return ErrDuplicate
			}
		}
		row, err := q.InsertChatMessage(ctx, db.InsertChatMessageParams{
			Channel: channel, UserID: userID, CharacterID: sender.CharacterID, CharacterName: sender.Name,
			StageID: sender.StageID, Body: body, RawBody: raw, IdempotencyKey: key, Now: now,
		})
		if err != nil {
			return err
		}
		out = fromRow(row)
		return nil
	})
	if err != nil {
		return Message{}, err
	}
	if !replay && s.Publish != nil {
		// After commit, outside the tx. At-most-once: a lost publish is recovered by the client's history fetch.
		_ = s.Publish(ctx, EventMessage, strconv.FormatInt(out.ID, 10), out)
	}
	return out, nil
}

func fromRow(r db.ChatMessage) Message {
	return Message{ID: r.ID, Channel: r.Channel, CharacterID: r.CharacterID, Name: r.CharacterName, StageID: r.StageID, Text: r.Body, SentAt: r.CreatedAt}
}

// History returns up to history_size visible messages before beforeID (0 = newest), oldest first.
func (s *Service) History(ctx context.Context, channel string, beforeID int64) ([]Message, error) {
	if _, ok := s.Cat.Channel(channel); !ok {
		return nil, ErrUnknownChannel
	}
	rows, err := db.New(s.Pool).ListChatMessages(ctx, db.ListChatMessagesParams{Channel: channel, BeforeID: beforeID, Lim: int32(s.Cat.HistorySize)})
	if err != nil {
		return nil, err
	}
	out := make([]Message, len(rows))
	for i, r := range rows { // rows are newest first → reverse
		out[len(rows)-1-i] = Message{ID: r.ID, Channel: r.Channel, CharacterID: r.CharacterID, Name: r.CharacterName,
			StageID: r.StageID, Text: r.Body, SentAt: r.CreatedAt}
	}
	return out, nil
}

// Hide removes a message from every client (admin). Idempotent: hiding a hidden message is a no-op. Audited.
func (s *Service) Hide(ctx context.Context, admin uuid.UUID, id int64, reason string) error {
	now := s.Now().UTC()
	var channel string
	changed := false
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		m, err := q.GetChatMessageForUpdate(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		channel = m.Channel
		if m.HiddenAt != nil {
			return nil
		}
		changed = true
		if err := q.HideChatMessage(ctx, db.HideChatMessageParams{Now: &now, Admin: pgtype.UUID{Bytes: admin, Valid: true}, ID: id}); err != nil {
			return err
		}
		return audit(ctx, q, admin, "chat.hide", "chat_message", strconv.FormatInt(id, 10),
			map[string]any{"body": m.Body, "characterId": m.CharacterID}, map[string]any{"hidden": true}, reason)
	})
	if err == nil && changed && s.Publish != nil {
		_ = s.Publish(ctx, EventHidden, "hide-"+strconv.FormatInt(id, 10), map[string]any{"id": id, "channel": channel})
	}
	return err
}

// MuteState is an account's mute.
type MuteState struct {
	UserID     uuid.UUID `json:"userId"`
	MutedUntil time.Time `json:"mutedUntil"`
}

// Mute blocks posting for hours (0 lifts the mute). Audited.
func (s *Service) Mute(ctx context.Context, admin, userID uuid.UUID, hours int, reason string) (MuteState, error) {
	if hours < 0 || time.Duration(hours)*time.Hour > s.Cat.MuteMax {
		return MuteState{}, ErrInvalidMute
	}
	now := s.Now().UTC()
	until := now.Add(time.Duration(hours) * time.Hour)
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		if _, err := q.LockUser(ctx, userID); errors.Is(err, pgx.ErrNoRows) {
			return ErrUnknownUser
		} else if err != nil {
			return err
		}
		before := map[string]any{"mutedUntil": nil}
		if prev, err := q.GetChatMuteForUpdate(ctx, userID); err == nil {
			before["mutedUntil"] = prev.MutedUntil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err := q.UpsertChatMute(ctx, db.UpsertChatMuteParams{UserID: userID, MutedUntil: until, Reason: reason, MutedBy: admin, Now: now}); err != nil {
			return err
		}
		return audit(ctx, q, admin, "chat.mute", "user", userID.String(), before, map[string]any{"mutedUntil": until, "hours": hours}, reason)
	})
	return MuteState{UserID: userID, MutedUntil: until}, err
}

// Sweep deletes messages older than retention in batches; returns rows deleted.
func (s *Service) Sweep(ctx context.Context) (int64, error) {
	before := s.Now().UTC().Add(-s.Cat.Retention)
	var total int64
	for {
		n, err := db.New(s.Pool).DeleteOldChatMessages(ctx, db.DeleteOldChatMessagesParams{Before: before, Lim: 5000})
		if err != nil {
			return total, fmt.Errorf("chat sweep: %w", err)
		}
		total += n
		if n < 5000 {
			return total, nil
		}
	}
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
