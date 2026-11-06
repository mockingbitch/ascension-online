// Package leaderboard encodes progression into a sortable score (GDD §7.2, §22).
package leaderboard

import "tiendao/game/internal/realm"

// Redis keys. Postgres is the source of truth; these are rebuildable caches.
const (
	KeyGlobalRealm = "leaderboard:global:realm"
)

// Score encodes (tier, stage order, progress ppm) into a float64 that is exact (< 2^53) and strictly ordered:
//
//	tier × 1e9 + stage_order × 1e6 + progress_ppm(0..999_999)
//
// It is path-agnostic (tiên and Vu at equal tier/stage rank equally; data-model §3.9). Stored cultivation above
// the requirement (bình cảnh) saturates at 999_999. Ties are broken outside Redis by stage_reached_at.
func Score(s *realm.Stage, cultivationMilli int64) float64 {
	ppm := int64(0)
	if s.RequiredMilli > 0 && cultivationMilli > 0 {
		if cultivationMilli >= s.RequiredMilli {
			ppm = 999_999
		} else {
			ppm = min(999_999, cultivationMilli*1_000_000/s.RequiredMilli)
		}
	}
	return float64(int64(s.Tier)*1_000_000_000 + int64(s.StageOrder)*1_000_000 + ppm)
}
