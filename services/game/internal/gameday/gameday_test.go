package gameday

import (
	"testing"
	"time"
)

func TestOf(t *testing.T) {
	// 2026-06-01 16:59 UTC = 23:59 ICT → still June 1; 17:00 UTC = 00:00 ICT June 2.
	a := Of(time.Date(2026, 6, 1, 16, 59, 0, 0, time.UTC))
	b := Of(time.Date(2026, 6, 1, 17, 0, 0, 0, time.UTC))
	if a.Day() != 1 || b.Day() != 2 {
		t.Fatalf("a=%v b=%v", a, b)
	}
	if !NextReset(time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)).Equal(time.Date(2026, 6, 1, 17, 0, 0, 0, time.UTC)) {
		t.Fatal("next reset")
	}
}
