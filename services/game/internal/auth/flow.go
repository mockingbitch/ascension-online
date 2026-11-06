package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// FlowState is carried in a signed, short-lived cookie between /start and /callback.
type FlowState struct {
	State    string    `json:"s"`
	Nonce    string    `json:"n"`
	Verifier string    `json:"v"` // PKCE code_verifier
	ReturnTo string    `json:"r"`
	Expires  time.Time `json:"e"`
}

// FlowSigner signs FlowState with HMAC-SHA256.
type FlowSigner struct{ key []byte }

// NewFlowSigner derives a dedicated key from the master secret.
func NewFlowSigner(secret []byte) FlowSigner {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("tiendao/oauth-flow/v1"))
	return FlowSigner{key: m.Sum(nil)}
}

// Encode serializes and signs.
func (f FlowSigner) Encode(s FlowState) string {
	b, _ := json.Marshal(s)
	payload := base64.RawURLEncoding.EncodeToString(b)
	return payload + "." + f.sign(payload)
}

// Decode verifies signature and expiry.
func (f FlowSigner) Decode(v string, now time.Time) (FlowState, error) {
	payload, sig, ok := strings.Cut(v, ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(f.sign(payload))) {
		return FlowState{}, errors.New("auth: bad flow signature")
	}
	b, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return FlowState{}, err
	}
	var s FlowState
	if err := json.Unmarshal(b, &s); err != nil {
		return FlowState{}, err
	}
	if now.After(s.Expires) {
		return FlowState{}, errors.New("auth: flow expired")
	}
	return s, nil
}

func (f FlowSigner) sign(payload string) string {
	m := hmac.New(sha256.New, f.key)
	m.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// RandomString returns n random bytes base64url-encoded.
func RandomString(n int) string {
	t, _ := newOpaqueTokenN(n)
	return t
}

func newOpaqueTokenN(n int) (string, error) {
	b := make([]byte, n)
	if _, err := randRead(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// SafeReturnPath accepts only same-origin relative paths ("/x"), rejecting "//host" and "/\host".
func SafeReturnPath(p string) string {
	if p == "" || p[0] != '/' || strings.HasPrefix(p, "//") || strings.HasPrefix(p, "/\\") || strings.ContainsAny(p, "\r\n") {
		return "/"
	}
	return p
}
