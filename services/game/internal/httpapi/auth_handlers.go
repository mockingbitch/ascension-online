package httpapi

import (
	"errors"
	"net/http"
	"time"

	"tiendao/game/internal/auth"
)

const (
	flowCookie    = "td_oauth"
	refreshCookie = "td_refresh"
	refreshPath   = "/api/v1/auth"
)

// googleStart: GET /auth/google/start?return=/path → 302 to Google with state, nonce and PKCE.
func (d Deps) googleStart(w http.ResponseWriter, r *http.Request) {
	if d.Google == nil {
		writeError(w, 503, "google_not_configured", "Google sign-in is not configured on this server")
		return
	}
	st := auth.FlowState{
		State: auth.RandomString(24), Nonce: auth.RandomString(24), Verifier: auth.GenerateVerifier(),
		ReturnTo: auth.SafeReturnPath(r.URL.Query().Get("return")), Expires: d.Now().Add(10 * time.Minute),
	}
	http.SetCookie(w, &http.Cookie{
		Name: flowCookie, Value: d.Flow.Encode(st), Path: "/api/v1/auth/google", MaxAge: 600,
		HttpOnly: true, Secure: d.SecureCookies, SameSite: http.SameSiteLaxMode, // Lax: must survive the top-level redirect back from Google
	})
	http.Redirect(w, r, d.Google.AuthURL(st.State, st.Nonce, st.Verifier), http.StatusFound)
}

// googleCallback: verifies state, exchanges code, logs in, sets the refresh cookie and returns to the app.
// The SPA then calls POST /auth/refresh to obtain its in-memory access token.
func (d Deps) googleCallback(w http.ResponseWriter, r *http.Request) {
	if d.Google == nil || d.Auth == nil {
		writeError(w, 503, "google_not_configured", "Google sign-in is not configured on this server")
		return
	}
	fail := func(reason string) {
		http.Redirect(w, r, d.PublicBaseURL+"/login?error="+reason, http.StatusFound)
	}
	c, err := r.Cookie(flowCookie)
	if err != nil {
		fail("session_expired")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: flowCookie, Path: "/api/v1/auth/google", MaxAge: -1, HttpOnly: true, Secure: d.SecureCookies})
	st, err := d.Flow.Decode(c.Value, d.Now())
	if err != nil || r.URL.Query().Get("state") != st.State {
		fail("invalid_state")
		return
	}
	if r.URL.Query().Get("error") != "" {
		fail("access_denied")
		return
	}
	id, err := d.Google.Exchange(r.Context(), r.URL.Query().Get("code"), st.Verifier, st.Nonce)
	if err != nil {
		d.Log.WarnContext(r.Context(), "google exchange failed", "err", err)
		fail("google_failed")
		return
	}
	sess, err := d.Auth.LoginWithIdentity(r.Context(), id, r.UserAgent())
	switch {
	case errors.Is(err, auth.ErrEmailUnverified):
		fail("email_unverified")
		return
	case errors.Is(err, auth.ErrEmailTaken):
		fail("email_taken")
		return
	case err != nil:
		d.Log.ErrorContext(r.Context(), "login failed", "err", err)
		fail("server_error")
		return
	}
	d.recordNet(r, sess.User.ID) // salted network hash (anti multi-account signal; never the raw IP)
	d.setRefreshCookie(w, sess)
	http.Redirect(w, r, d.PublicBaseURL+st.ReturnTo, http.StatusFound)
}

type sessionResponse struct {
	AccessToken string    `json:"accessToken"`
	ExpiresIn   int64     `json:"expiresIn"` // seconds
	User        auth.User `json:"user"`
}

// refresh: POST /auth/refresh (cookie) → new access token + rotated refresh cookie.
// CSRF: the cookie is SameSite=Strict and the request must carry X-Requested-With (forces CORS preflight).
func (d Deps) refresh(w http.ResponseWriter, r *http.Request) {
	if d.Auth == nil {
		unavailable(w)
		return
	}
	if r.Header.Get("X-Requested-With") != "tiendao" {
		writeError(w, 400, "missing_header", "X-Requested-With: tiendao is required")
		return
	}
	c, err := r.Cookie(refreshCookie)
	if err != nil || c.Value == "" {
		writeError(w, 401, "no_session", "not signed in")
		return
	}
	sess, err := d.Auth.Refresh(r.Context(), c.Value, r.UserAgent())
	if errors.Is(err, auth.ErrInvalidRefresh) || errors.Is(err, auth.ErrRefreshReused) {
		if errors.Is(err, auth.ErrRefreshReused) {
			d.Log.WarnContext(r.Context(), "refresh token reuse detected; family revoked")
		}
		d.clearRefreshCookie(w)
		writeError(w, 401, "session_expired", "please sign in again")
		return
	} else if err != nil {
		d.internal(w, r, "refresh", err)
		return
	}
	d.recordNet(r, sess.User.ID)
	d.setRefreshCookie(w, sess)
	writeJSON(w, 200, sessionResponse{AccessToken: sess.AccessToken, ExpiresIn: int64(sess.AccessExpiresIn / time.Second), User: sess.User})
}

func (d Deps) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(refreshCookie); err == nil && d.Auth != nil {
		if err := d.Auth.Logout(r.Context(), c.Value); err != nil {
			d.internal(w, r, "logout", err)
			return
		}
	}
	d.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (d Deps) me(w http.ResponseWriter, r *http.Request) {
	if d.Auth == nil {
		unavailable(w)
		return
	}
	u, err := d.Auth.Me(r.Context(), userFrom(r.Context()))
	if err != nil {
		d.internal(w, r, "me", err)
		return
	}
	writeJSON(w, 200, u)
}

func (d Deps) setRefreshCookie(w http.ResponseWriter, s auth.Session) {
	http.SetCookie(w, &http.Cookie{
		Name: refreshCookie, Value: s.RefreshToken, Path: refreshPath, Expires: s.RefreshExpiresAt,
		HttpOnly: true, Secure: d.SecureCookies, SameSite: http.SameSiteStrictMode,
	})
}

func (d Deps) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: refreshCookie, Path: refreshPath, MaxAge: -1, HttpOnly: true, Secure: d.SecureCookies, SameSite: http.SameSiteStrictMode})
}
