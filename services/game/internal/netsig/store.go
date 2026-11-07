package netsig

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/db"
)

// Store records and queries network hashes in Postgres (net_signals).
type Store struct {
	Pool   *pgxpool.Pool
	Hasher *Hasher
	Now    func() time.Time
}

// Record stores the account's hash of ip under every key (upsert: first/last seen). Callers treat errors as best
// effort (a missing signal must never break a login).
func (s *Store) Record(ctx context.Context, userID uuid.UUID, ip string) error {
	if s == nil || s.Hasher == nil {
		return nil
	}
	hs, err := s.Hasher.Hash(ip)
	if err != nil {
		return err
	}
	now := s.Now().UTC()
	q := db.New(s.Pool)
	for _, h := range hs {
		if err := q.UpsertNetSignal(ctx, db.UpsertNetSignalParams{UserID: userID, KeyID: h.KeyID, IpHash: h.Sum, SeenAt: now}); err != nil {
			return err
		}
	}
	return nil
}

// SharedRecently reports whether two accounts were seen on the same network (same hash, same salt key) within the
// retention window. A nil Store answers false.
func (s *Store) SharedRecently(ctx context.Context, a, b uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil || a == b {
		return false, nil
	}
	return SharedRecentlyQ(ctx, db.New(s.Pool), a, b, s.Now())
}

// SharedRecentlyQ is SharedRecently on a caller's queries (e.g. inside a transaction).
func SharedRecentlyQ(ctx context.Context, q *db.Queries, a, b uuid.UUID, now time.Time) (bool, error) {
	if a == b {
		return false, nil
	}
	return q.NetSignalsShared(ctx, db.NetSignalsSharedParams{UserA: a, UserB: b, Since: now.UTC().Add(-Retention)})
}

// Account is one account of a cluster (no hash, no IP).
type Account struct {
	UserID        string `json:"userId"`
	Email         string `json:"email"`
	CharacterID   string `json:"characterId,omitempty"`
	CharacterName string `json:"characterName,omitempty"`
}

// Cluster is a set of accounts seen on the same network(s) within the window.
type Cluster struct {
	Accounts       []Account `json:"accounts"`
	SharedNetworks int       `json:"sharedNetworks"` // distinct network hashes this exact set shared
	LastSeenAt     time.Time `json:"lastSeenAt"`
}

// Clusters lists account sets sharing a network hash within the last `days` (≤ RetentionDays), largest first.
// Sets equal across several hashes (or salt keys during a rotation) are merged. Admin only.
func (s *Store) Clusters(ctx context.Context, days, minAccounts, limit int) ([]Cluster, error) {
	days = max(1, min(days, RetentionDays))
	minAccounts = max(2, minAccounts)
	limit = max(1, min(limit, 200))
	q := db.New(s.Pool)
	rows, err := q.ListNetSignalGroups(ctx, db.ListNetSignalGroupsParams{Since: s.Now().UTC().AddDate(0, 0, -days),
		MinAccounts: int64(minAccounts), Lim: int32(limit * 4)})
	if err != nil {
		return nil, err
	}
	type agg struct {
		users  []uuid.UUID
		hashes map[string]bool
		last   time.Time
	}
	byKey := map[string]*agg{}
	order := []string{}
	all := map[uuid.UUID]bool{}
	for _, r := range rows {
		ids := slices.Clone(r.Users)
		slices.SortFunc(ids, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
		parts := make([]string, len(ids))
		for i, id := range ids {
			parts[i] = id.String()
			all[id] = true
		}
		k := strings.Join(parts, ",")
		a, ok := byKey[k]
		if !ok {
			a = &agg{users: ids, hashes: map[string]bool{}}
			byKey[k] = a
			order = append(order, k)
		}
		// Count networks, not salt keys: the same network under two keys is one network
		// keep per-key counts and report the max over keys.
		a.hashes[r.KeyID+"|"+string(r.IpHash)] = true
		if r.LastSeenAt.After(a.last) {
			a.last = r.LastSeenAt
		}
	}
	ids := make([]uuid.UUID, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}
	briefs := map[uuid.UUID]db.ListUserBriefsRow{}
	if len(ids) > 0 {
		bs, err := q.ListUserBriefs(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, b := range bs {
			briefs[b.UserID] = b
		}
	}
	out := make([]Cluster, 0, len(order))
	for _, k := range order {
		if len(out) >= limit {
			break
		}
		a := byKey[k]
		perKey := map[string]int{}
		for h := range a.hashes {
			id, _, _ := strings.Cut(h, "|")
			perKey[id]++
		}
		n := 0
		for _, c := range perKey {
			n = max(n, c)
		}
		c := Cluster{SharedNetworks: n, LastSeenAt: a.last, Accounts: make([]Account, 0, len(a.users))}
		for _, u := range a.users {
			b := briefs[u]
			acc := Account{UserID: u.String(), Email: b.Email}
			if b.CharacterID.Valid {
				acc.CharacterID = uuid.UUID(b.CharacterID.Bytes).String()
				acc.CharacterName = b.CharacterName.String
			}
			c.Accounts = append(c.Accounts, acc)
		}
		out = append(out, c)
	}
	return out, nil
}

// Purge deletes rows unseen for RetentionDays in batches of batch, at most maxBatches per call (the worker calls it
// periodically, so a backlog drains over several runs). Returns the rows deleted.
func (s *Store) Purge(ctx context.Context, batch, maxBatches int) (int64, error) {
	cutoff := s.Now().UTC().Add(-Retention)
	q := db.New(s.Pool)
	var total int64
	for range max(1, maxBatches) {
		n, err := q.PurgeNetSignals(ctx, db.PurgeNetSignalsParams{Cutoff: cutoff, Lim: int32(batch)})
		total += n
		if err != nil || n < int64(batch) {
			return total, err
		}
	}
	return total, nil
}

// SharedRecentlyCharacters is SharedRecently for two characters (resolved to their accounts), e.g. hộ pháp pairs.
// Additive helper; a nil Store answers false.
func (s *Store) SharedRecentlyCharacters(ctx context.Context, a, b uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil || a == b {
		return false, nil
	}
	return db.New(s.Pool).NetSignalsSharedByCharacters(ctx, db.NetSignalsSharedByCharactersParams{CharacterA: a, CharacterB: b,
		Since: s.Now().UTC().Add(-Retention)})
}

// AuditRead records an admin's read of network-signal data (admin_audit_logs, review L7). Call before returning data.
func (s *Store) AuditRead(ctx context.Context, adminUserID uuid.UUID, what string, params map[string]any) error {
	after, _ := json.Marshal(params)
	return db.New(s.Pool).InsertAdminAudit(ctx, db.InsertAdminAuditParams{AdminUserID: adminUserID, Action: what + ".read",
		TargetType: "admin_read", TargetID: what, After: after})
}
