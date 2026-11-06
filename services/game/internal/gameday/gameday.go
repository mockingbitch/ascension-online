// Package gameday defines the game's business day: midnight-to-midnight in Vietnam time (UTC+7, no DST).
// Daily resets (tasks, VIP gift, Tụ Linh Phù limit) all use this.
package gameday

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Zone is the game's reset timezone.
var Zone = time.FixedZone("ICT", 7*3600)

// Of returns the game day (00:00 in Zone, expressed as a UTC date value) containing t.
func Of(t time.Time) time.Time {
	y, m, d := t.In(Zone).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// PG converts the game day of t to a pgtype.Date.
func PG(t time.Time) pgtype.Date { return pgtype.Date{Time: Of(t), Valid: true} }

// Same reports whether a stored date equals the game day of t.
func Same(d pgtype.Date, t time.Time) bool { return d.Valid && d.Time.Equal(Of(t)) }

// NextReset returns the next 00:00 in Zone after t.
func NextReset(t time.Time) time.Time {
	day := Of(t)
	return time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, Zone)
}
