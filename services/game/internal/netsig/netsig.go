// Package netsig is the anti-multi-account network signal (decisions 2026-11-07 "Chủ dự án chốt trước Phase 4–5"):
// at Google login and session refresh the API stores HMAC-SHA256(secret salt, client IP) truncated to 16 bytes per
// account — never the raw IP — and the worker deletes rows unseen for RetentionDays. The signal only ever WARNS admins
// about accounts on the same network (đạo lữ song tu, gifts/trades, hộ pháp); it never blocks a player and the hashes
// are never returned by any API.
//
// Reuse (Phase 5.2 trades etc.): call Store.SharedRecently(ctx, userA, userB) before/inside a feature's flow and record
// a flag for admins; list clusters with Store.Clusters.
package netsig

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strings"
	"time"
)

// RetentionDays is how long a network hash is kept after it was last seen (product decision; privacy policy).
const RetentionDays = 30

// Retention is RetentionDays as a duration.
const Retention = RetentionDays * 24 * time.Hour

// HashLen is the stored (truncated) hash length in bytes.
const HashLen = 16

// MinSecretLen is the minimum salt length outside dev.
const MinSecretLen = 32

// Key is one named salt.
type Key struct {
	ID     string
	Secret []byte
}

// Hasher turns client IPs into salted hashes under every configured key. Keys[0] is the current salt; extra keys are
// kept during a rotation (each login is hashed under all of them, so matches survive the switch; drop the old key
// after RetentionDays).
type Hasher struct {
	Keys []Key
}

var keyIDRe = regexp.MustCompile(`^[a-z0-9]{1,16}$`)

// ParseKeys reads NETSIG_SALTS: "id:secret[,id:secret…]" (first = current). minLen is the minimum secret length.
func ParseKeys(spec string, minLen int) ([]Key, error) {
	var out []Key
	seen := map[string]bool{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, secret, ok := strings.Cut(part, ":")
		if !ok || !keyIDRe.MatchString(id) {
			return nil, fmt.Errorf("netsig: salt entry must be <id>:<secret> with id [a-z0-9]{1,16}")
		}
		if seen[id] {
			return nil, fmt.Errorf("netsig: duplicate salt id %q", id)
		}
		if len(secret) < minLen {
			return nil, fmt.Errorf("netsig: salt %q shorter than %d bytes", id, minLen)
		}
		seen[id] = true
		out = append(out, Key{ID: id, Secret: []byte(secret)})
	}
	if len(out) == 0 {
		return nil, errors.New("netsig: no salt configured")
	}
	return out, nil
}

// ErrBadIP is returned for an unparsable client address.
var ErrBadIP = errors.New("netsig: unparsable client IP")

// Normalize reduces an address to what identifies a network: the full IPv4 address (IPv4-mapped IPv6 unmapped) or the
// /64 prefix of an IPv6 address (one household/LAN usually shares a /64 while devices rotate the interface id).
func Normalize(ip string) ([]byte, error) {
	a, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return nil, ErrBadIP
	}
	a = a.Unmap().WithZone("")
	if a.Is4() {
		b := a.As4()
		return append([]byte("4|"), b[:]...), nil
	}
	b := a.As16()
	return append([]byte("6|"), b[:8]...), nil
}

// Hash is one keyed hash of a client address.
type Hash struct {
	KeyID string
	Sum   []byte // HashLen bytes
}

// Hash returns the client's network hash under every key. Pure.
func (h *Hasher) Hash(ip string) ([]Hash, error) {
	n, err := Normalize(ip)
	if err != nil {
		return nil, err
	}
	out := make([]Hash, 0, len(h.Keys))
	for _, k := range h.Keys {
		m := hmac.New(sha256.New, k.Secret)
		m.Write([]byte("netsig/v1|"))
		m.Write(n)
		out = append(out, Hash{KeyID: k.ID, Sum: m.Sum(nil)[:HashLen]})
	}
	return out, nil
}
