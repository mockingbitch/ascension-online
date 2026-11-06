package auth

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

	"tiendao/game/internal/db"
	"tiendao/game/internal/platform/postgres"
)

var (
	ErrEmailUnverified = errors.New("auth: email not verified by provider")
	ErrEmailTaken      = errors.New("auth: email already linked to another account")
	ErrInvalidRefresh  = errors.New("auth: invalid refresh token")
	ErrRefreshReused   = errors.New("auth: refresh token reuse detected")
)

// RefreshGrace is how long a just-rotated refresh token is still accepted (concurrent tabs/retries).
const RefreshGrace = 20 * time.Second

// User is the public account view.
type User struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	AvatarURL   string    `json:"avatarUrl"`
	Role        string    `json:"role"`
}

// Session is returned after login/refresh. RefreshToken goes into an HttpOnly cookie, never JSON.
type Session struct {
	AccessToken      string
	AccessExpiresIn  time.Duration
	RefreshToken     string
	RefreshExpiresAt time.Time
	User             User
}

// Service owns accounts, identities and refresh tokens.
type Service struct {
	Pool       *pgxpool.Pool
	Issuer     *Issuer
	RefreshTTL time.Duration
	Now        func() time.Time
}

// LoginWithIdentity finds or creates the user for a verified identity and starts a new session family.
func (s *Service) LoginWithIdentity(ctx context.Context, id Identity, userAgent string) (Session, error) {
	sess, err := s.loginOnce(ctx, id, userAgent)
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" && pe.ConstraintName == "user_identities_pkey" {
		return s.loginOnce(ctx, id, userAgent) // lost a race creating the same identity: now it exists
	}
	return sess, err
}

func (s *Service) loginOnce(ctx context.Context, id Identity, userAgent string) (Session, error) {
	if !id.EmailVerified {
		return Session{}, ErrEmailUnverified
	}
	var sess Session
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		var u db.User
		ident, err := q.GetIdentity(ctx, db.GetIdentityParams{Provider: id.Provider, Subject: id.Subject})
		switch {
		case err == nil:
			if u, err = q.GetUser(ctx, ident.UserID); err != nil {
				return err
			}
			if err := q.TouchIdentity(ctx, db.TouchIdentityParams{Provider: id.Provider, Subject: id.Subject, Email: id.Email, EmailVerified: id.EmailVerified}); err != nil {
				return err
			}
			if err := q.TouchUserLogin(ctx, db.TouchUserLoginParams{ID: u.ID, DisplayName: text(id.Name), AvatarUrl: text(id.Picture)}); err != nil {
				return err
			}
		case errors.Is(err, pgx.ErrNoRows):
			// New identity. Never auto-link to an existing account by email (account-takeover risk).
			if _, err := q.GetUserByEmail(ctx, id.Email); err == nil {
				return ErrEmailTaken
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if u, err = q.CreateUser(ctx, db.CreateUserParams{Email: id.Email, DisplayName: text(id.Name), AvatarUrl: text(id.Picture)}); err != nil {
				return err
			}
			if err := q.InsertIdentity(ctx, db.InsertIdentityParams{Provider: id.Provider, Subject: id.Subject, UserID: u.ID, Email: id.Email, EmailVerified: id.EmailVerified}); err != nil {
				return err
			}
			if err := q.EnsureVIPAccount(ctx, u.ID); err != nil {
				return err
			}
		default:
			return err
		}
		sess, _, err = s.issue(ctx, q, u, uuid.New(), userAgent)
		return err
	})
	return sess, err
}

// Refresh rotates a refresh token. Presenting an already-rotated/revoked token revokes the whole family.
func (s *Service) Refresh(ctx context.Context, token, userAgent string) (Session, error) {
	var sess Session
	var reused bool
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		rt, err := q.GetRefreshTokenForUpdate(ctx, hashToken(token))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidRefresh
		} else if err != nil {
			return err
		}
		if rt.RevokedAt != nil {
			// Grace window: another tab / a retried request rotated this token moments ago.
			// Issue a sibling token in the same family instead of treating it as theft.
			if rt.ReplacedBy.Valid && rt.LastUsedAt != nil && s.Now().Sub(*rt.LastUsedAt) < RefreshGrace && s.Now().Before(rt.ExpiresAt) {
				u, err := q.GetUser(ctx, rt.UserID)
				if err != nil {
					return err
				}
				sess, _, err = s.issue(ctx, q, u, rt.FamilyID, userAgent)
				return err
			}
			if rt.ReplacedBy.Valid { // a rotated token came back later → likely stolen
				reused = true
				return q.RevokeRefreshFamily(ctx, rt.FamilyID) // commit the revocation
			}
			return ErrInvalidRefresh
		}
		if !s.Now().Before(rt.ExpiresAt) {
			return ErrInvalidRefresh
		}
		u, err := q.GetUser(ctx, rt.UserID)
		if err != nil {
			return err
		}
		var newID uuid.UUID
		if sess, newID, err = s.issue(ctx, q, u, rt.FamilyID, userAgent); err != nil {
			return err
		}
		return q.RotateRefreshToken(ctx, db.RotateRefreshTokenParams{ID: rt.ID, ReplacedBy: pgtype.UUID{Bytes: newID, Valid: true}})
	})
	if err == nil && reused {
		return Session{}, ErrRefreshReused
	}
	return sess, err
}

// Logout revokes the token's whole family. Unknown tokens are ignored.
func (s *Service) Logout(ctx context.Context, token string) error {
	return postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		rt, err := q.GetRefreshTokenForUpdate(ctx, hashToken(token))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}
		return q.RevokeRefreshFamily(ctx, rt.FamilyID)
	})
}

// Me returns the user's public view.
func (s *Service) Me(ctx context.Context, userID uuid.UUID) (User, error) {
	u, err := db.New(s.Pool).GetUser(ctx, userID)
	if err != nil {
		return User{}, err
	}
	return publicUser(u), nil
}

func (s *Service) issue(ctx context.Context, q *db.Queries, u db.User, family uuid.UUID, ua string) (Session, uuid.UUID, error) {
	access, err := s.Issuer.Issue(Principal{UserID: u.ID, Role: u.Role})
	if err != nil {
		return Session{}, uuid.Nil, err
	}
	token, hash := newOpaqueToken()
	exp := s.Now().Add(s.RefreshTTL)
	if len(ua) > 256 {
		ua = ua[:256]
	}
	id, err := q.InsertRefreshToken(ctx, db.InsertRefreshTokenParams{UserID: u.ID, TokenHash: hash, FamilyID: family, ExpiresAt: exp, UserAgent: text(ua)})
	if err != nil {
		return Session{}, uuid.Nil, fmt.Errorf("insert refresh: %w", err)
	}
	return Session{AccessToken: access, AccessExpiresIn: s.Issuer.TTL(), RefreshToken: token, RefreshExpiresAt: exp, User: publicUser(u)}, id, nil
}

func publicUser(u db.User) User {
	return User{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName.String, AvatarURL: u.AvatarUrl.String, Role: u.Role}
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
