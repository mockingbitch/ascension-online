package cultivation

import (
	"tiendao/game/internal/contribution"
	"tiendao/game/internal/realm"
)

// Progress is the cultivation part of a progression row.
type Progress struct {
	Stage             *realm.Stage
	StoredMilli       int64 // cultivation stored in the current stage
	GrantReserveMilli int64 // late-join grant not yet released
}

// CreditResult is the pure outcome of one credit (shared by CreditLocked and the read-only projection).
type CreditResult struct {
	Applied            contribution.Applied
	After              Progress
	Passed             []*realm.Stage // stages left by auto-advance (credit and grant release)
	GrantReleasedMilli int64
	BottleneckRoom     int64 // room left after the credit
}

// Credit runs the single credit path in memory (§2.4):
// raw → ×catch-up → bucket/ceiling caps → bottleneck room → stored += credited → auto-advance minor stages →
// release late-join reserve into the remaining bottleneck room → auto-advance again. Pure.
func Credit(cat *realm.Catalogue, cfg *contribution.Config, p Progress, day contribution.Day,
	src contribution.Source, rawMilli int64, catchUp bool) CreditResult {
	room := cat.BottleneckRoomMilli(p.Stage, p.StoredMilli)
	a := cfg.Apply(src, rawMilli, day, catchUp, room)
	stage, stored, passed := cat.Advance(p.Stage, p.StoredMilli+a.CreditedMilli)
	r := CreditResult{Applied: a, Passed: passed}
	r.After, r.GrantReleasedMilli, passed = ReleaseGrant(cat, Progress{Stage: stage, StoredMilli: stored, GrantReserveMilli: p.GrantReserveMilli})
	r.Passed = append(r.Passed, passed...)
	r.BottleneckRoom = cat.BottleneckRoomMilli(r.After.Stage, r.After.StoredMilli)
	return r
}

// ReleaseGrant moves min(reserve, bottleneck room) into stored cultivation and auto-advances. The grant fills a
// realm but never skips a major breakthrough, its items or a tribulation (C9). Pure.
func ReleaseGrant(cat *realm.Catalogue, p Progress) (Progress, int64, []*realm.Stage) {
	if p.GrantReserveMilli <= 0 {
		return p, 0, nil
	}
	rel := min(p.GrantReserveMilli, cat.BottleneckRoomMilli(p.Stage, p.StoredMilli))
	if rel <= 0 {
		return p, 0, nil
	}
	stage, stored, passed := cat.Advance(p.Stage, p.StoredMilli+rel)
	return Progress{Stage: stage, StoredMilli: stored, GrantReserveMilli: p.GrantReserveMilli - rel}, rel, passed
}
