package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Identity is a verified external identity.
type Identity struct {
	Provider      string
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

// Provider abstracts an OIDC provider so handlers can be tested without Google.
type Provider interface {
	AuthURL(state, nonce, verifier string) string
	Exchange(ctx context.Context, code, verifier, nonce string) (Identity, error)
}

// Google implements Provider with Authorization Code + PKCE and ID-token verification.
type Google struct {
	cfg      oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// NewGoogle performs OIDC discovery (network call) — do it once at startup.
func NewGoogle(ctx context.Context, clientID, clientSecret, redirectURL string) (*Google, error) {
	p, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, fmt.Errorf("auth: google discovery: %w", err)
	}
	return &Google{
		cfg: oauth2.Config{
			ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL,
			Endpoint: p.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: p.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}

// AuthURL builds the consent redirect.
func (g *Google) AuthURL(state, nonce, verifier string) string {
	return g.cfg.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("prompt", "select_account"))
}

// Exchange trades the code for tokens and verifies the ID token (signature, iss, aud, exp, nonce).
func (g *Google) Exchange(ctx context.Context, code, verifier, nonce string) (Identity, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: 10 * time.Second})
	tok, err := g.cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("auth: code exchange: %w", err)
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok {
		return Identity{}, errors.New("auth: no id_token")
	}
	idt, err := g.verifier.Verify(ctx, raw)
	if err != nil {
		return Identity{}, fmt.Errorf("auth: id_token: %w", err)
	}
	if idt.Nonce != nonce {
		return Identity{}, errors.New("auth: nonce mismatch")
	}
	var c struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := idt.Claims(&c); err != nil {
		return Identity{}, err
	}
	return Identity{Provider: "google", Subject: idt.Subject, Email: c.Email, EmailVerified: c.EmailVerified, Name: c.Name, Picture: c.Picture}, nil
}

// GenerateVerifier returns a PKCE code_verifier.
func GenerateVerifier() string { return oauth2.GenerateVerifier() }
