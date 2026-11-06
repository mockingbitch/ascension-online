package world

import (
	"path/filepath"

	"github.com/jackc/pgx/v5/pgtype"
	"testing"
	"time"

	"tiendao/game/internal/gameday"
)

func TestShippedWorldIsValid(t *testing.T) {
	c, err := LoadFile(filepath.Join("..", "..", "..", "..", "content", "catalog", "world_milestones.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Settings.WorldCreditCapMilli != 100_000 || c.Settings.ReferencePerActiveMilli != 67_000 || c.Settings.MedianWindowDays != 7 {
		t.Fatalf("settings %+v", c.Settings)
	}
}

func TestCountablePerCapitaMedian(t *testing.T) {
	if Countable(150_000, 0, 100_000) != 100_000 || Countable(80_000, 30_000, 100_000) != 50_000 || Countable(10, 20, 100) != 0 {
		t.Fatal("countable")
	}
	if PerCapita(1000, 0) != 0 || PerCapita(1000, 3) != 333 {
		t.Fatal("per capita")
	}
	if Median(nil) != 0 || Median([]int64{5, 1, 3}) != 3 || Median([]int64{4, 1, 3, 2}) != 2 {
		t.Fatal("median")
	}
}

func TestServerDayAndClose(t *testing.T) {
	start := time.Date(2027, 1, 10, 1, 0, 0, 0, time.UTC) // 08:00 ICT Jan 10
	if ServerDay(nil, start) != -1 || ServerDay(&start, start.Add(-time.Second)) != -1 {
		t.Fatal("not started")
	}
	if ServerDay(&start, start) != 0 || ServerDay(&start, time.Date(2027, 1, 10, 17, 0, 0, 0, time.UTC)) != 1 {
		t.Fatal("day boundary at 00:00 ICT")
	}
	day := gameday.Of(start)
	want := time.Date(2027, 1, 10, 17, 5, 0, 0, time.UTC)
	if got := ClosableAt(day, 5*time.Minute); !got.Equal(want) {
		t.Fatalf("closable %v want %v", got, want)
	}
}

func TestDaysToProcess(t *testing.T) {
	now := time.Date(2026, 6, 1, 17, 10, 0, 0, time.UTC) // 2026-06-02 game time
	d := func(s string) time.Time { v, _ := time.Parse(time.DateOnly, s); return v }
	fmtDays := func(ds []time.Time) (out []string) {
		for _, x := range ds {
			out = append(out, x.Format(time.DateOnly))
		}
		return
	}
	if got := fmtDays(DaysToProcess(pgtype.Date{}, nil, now)); len(got) != 2 || got[0] != "2026-06-01" {
		t.Fatalf("pre-launch: %v", got)
	}
	start := time.Date(2026, 5, 29, 1, 0, 0, 0, time.UTC)
	if got := fmtDays(DaysToProcess(pgtype.Date{}, &start, now)); len(got) != 5 || got[0] != "2026-05-29" {
		t.Fatalf("since launch: %v", got)
	}
	if got := DaysToProcess(pgtype.Date{Time: d("2026-06-02"), Valid: true}, &start, now); len(got) != 1 {
		t.Fatalf("closed through today: %v", got)
	}
	if got := DaysToProcess(pgtype.Date{Time: d("2020-01-01"), Valid: true}, nil, now); len(got) != MaxCatchUpDays {
		t.Fatalf("bounded: %d", len(got))
	}
}
