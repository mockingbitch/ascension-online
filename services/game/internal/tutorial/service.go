package tutorial

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/db"
	"tiendao/game/internal/platform/postgres"
)

var (
	// ErrNoCharacter: the account has no character yet (onboarding steps are client-only).
	ErrNoCharacter = errors.New("tutorial: no character")
	// ErrUnknownStep: not a stored (play) step of the scripts.
	ErrUnknownStep = errors.New("tutorial: unknown step")
	// ErrBadStatus: status is neither done nor skipped.
	ErrBadStatus = errors.New("tutorial: status must be done or skipped")
	// ErrKeyReuse: the Idempotency-Key already marked another step.
	ErrKeyReuse = errors.New("tutorial: idempotency key reused")
)

// State is GET /tutorial and the answer of every mutation (the client replaces, never merges).
type State struct {
	Done    []string `json:"done"`
	Skipped []string `json:"skipped"`
	// Muted: "không nhắc lại" — the guide offers no step on her own (the "?" button still summons her).
	Muted bool `json:"muted"`
}

// MarkResult is POST /tutorial/steps/{stepId}.
type MarkResult struct {
	Replayed bool  `json:"replayed"`
	State    State `json:"state"`
}

// Service stores the tutorial progress of a character.
type Service struct {
	Pool   *pgxpool.Pool
	Script *Script
	Now    func() time.Time
}

func (s *Service) character(ctx context.Context, q *db.Queries, userID uuid.UUID) (uuid.UUID, error) {
	ch, err := q.GetCharacterByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNoCharacter
	}
	return ch.ID, err
}

// lock takes the per-character tutorial lock (the prefs row, created on first use): every write serialises here, so
// two devices or a double tap never race on the idempotency key.
func (s *Service) lock(ctx context.Context, q *db.Queries, userID uuid.UUID) (uuid.UUID, db.LockTutorialPrefsRow, error) {
	charID, err := s.character(ctx, q, userID)
	if err != nil {
		return uuid.Nil, db.LockTutorialPrefsRow{}, err
	}
	if err := q.EnsureTutorialPrefs(ctx, charID); err != nil {
		return uuid.Nil, db.LockTutorialPrefsRow{}, err
	}
	row, err := q.LockTutorialPrefs(ctx, charID)
	if err != nil {
		return uuid.Nil, db.LockTutorialPrefsRow{}, fmt.Errorf("lock tutorial: %w", err)
	}
	return charID, row, nil
}

func (s *Service) state(ctx context.Context, q *db.Queries, charID uuid.UUID) (State, error) {
	st := State{Done: []string{}, Skipped: []string{}}
	rows, err := q.ListTutorialProgress(ctx, charID)
	if err != nil {
		return State{}, err
	}
	for _, r := range rows {
		if !s.Script.Stored(r.StepID) {
			continue // a retired step: ignore
		}
		if r.Status == "skipped" {
			st.Skipped = append(st.Skipped, r.StepID)
		} else {
			st.Done = append(st.Done, r.StepID)
		}
	}
	muted, err := q.GetTutorialMuted(ctx, charID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return State{}, err
	}
	st.Muted = muted
	return st, nil
}

// Get returns the caller's progress.
func (s *Service) Get(ctx context.Context, userID uuid.UUID) (State, error) {
	q := db.New(s.Pool)
	charID, err := s.character(ctx, q, userID)
	if err != nil {
		return State{}, err
	}
	return s.state(ctx, q, charID)
}

// Mark records that the character saw a step (done) or dismissed it (skipped). Idempotent: the first mark of a step
// wins, the same key replays, a key used for another step is refused. No side effect beyond the row.
func (s *Service) Mark(ctx context.Context, userID uuid.UUID, stepID, status, key string) (MarkResult, error) {
	if !s.Script.Stored(stepID) {
		return MarkResult{}, fmt.Errorf("%w: %q", ErrUnknownStep, stepID)
	}
	if status != "done" && status != "skipped" {
		return MarkResult{}, ErrBadStatus
	}
	var out MarkResult
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		charID, _, err := s.lock(ctx, q, userID)
		if err != nil {
			return err
		}
		if prev, err := q.GetTutorialProgressByKey(ctx, db.GetTutorialProgressByKeyParams{CharacterID: charID, IdempotencyKey: key}); err == nil {
			if prev.StepID != stepID {
				return ErrKeyReuse
			}
			out.Replayed = true
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		} else if _, err := q.InsertTutorialProgress(ctx, db.InsertTutorialProgressParams{CharacterID: charID, StepID: stepID,
			Status: status, IdempotencyKey: key, Now: s.Now()}); err != nil {
			return fmt.Errorf("mark step: %w", err)
		}
		out.State, err = s.state(ctx, q, charID)
		return err
	})
	return out, err
}

// SetMuted turns "không nhắc lại" on or off. A retry with the last settings key replays (nothing re-applied).
func (s *Service) SetMuted(ctx context.Context, userID uuid.UUID, muted bool, key string) (State, error) {
	return s.setting(ctx, userID, key, func(q *db.Queries, charID uuid.UUID) error {
		return q.SetTutorialMuted(ctx, db.SetTutorialMutedParams{CharacterID: charID, Muted: muted, LastKey: pgtype.Text{String: key, Valid: true}, Now: s.Now()})
	})
}

// setting runs one settings action under the lock, once per key: the same key as the last action = replay.
func (s *Service) setting(ctx context.Context, userID uuid.UUID, key string, apply func(*db.Queries, uuid.UUID) error) (State, error) {
	var out State
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		charID, row, err := s.lock(ctx, q, userID)
		if err != nil {
			return err
		}
		if !row.LastKey.Valid || row.LastKey.String != key {
			if err := apply(q, charID); err != nil {
				return err
			}
		}
		out, err = s.state(ctx, q, charID)
		return err
	})
	return out, err
}

// Reset forgets every seen step and un-mutes the guide (Cài đặt › "Xem lại hướng dẫn"). A retry with the same key
// replays — a lost answer never wipes steps marked since on another device. (An old Mark key replayed after a reset
// marks its step again: accepted, display only.)
func (s *Service) Reset(ctx context.Context, userID uuid.UUID, key string) (State, error) {
	return s.setting(ctx, userID, key, func(q *db.Queries, charID uuid.UUID) error {
		if err := q.DeleteTutorialProgress(ctx, charID); err != nil {
			return err
		}
		return q.SetTutorialMuted(ctx, db.SetTutorialMutedParams{CharacterID: charID, Muted: false, LastKey: pgtype.Text{String: key, Valid: true}, Now: s.Now()})
	})
}
