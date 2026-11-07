package dailytask

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"tiendao/game/internal/db"
)

func TestShippedDailyTasksAreValid(t *testing.T) {
	c, err := LoadFile(filepath.Join("..", "..", "..", "..", "content", "catalog", "daily_tasks.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get("trung_kich"); ok {
		t.Fatal("trung_kich must be replaced by nghe_giang (decision 13)")
	}
	if t2, ok := c.ByKind(LectureAttend); !ok || t2.ID != "nghe_giang" {
		t.Fatal("nghe_giang missing")
	}
	if _, ok := c.ByKind(Incense); !ok {
		t.Fatal("incense task missing")
	}
}

func load(t *testing.T, y string) error {
	t.Helper()
	p := filepath.Join(t.TempDir(), "d.yaml")
	if err := os.WriteFile(p, []byte(y), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFile(p)
	return err
}

func TestRejectsBadIncense(t *testing.T) {
	err := load(t, `version: 1
tasks:
  - { id: a, name: A, description: d, kind: incense, target: 2, streak_rewards: [{ min_streak: 2, points_milli: 0 }, { min_streak: 2, points_milli: 99999 }] }
  - { id: b, name: B, description: d, kind: incense, target: 1 }
  - { id: c, name: C, description: d, kind: checkin, target: 1, rewards: [{ currency: spirit_stone, amount: 1 }], streak_rewards: [{ min_streak: 1, points_milli: 1 }] }
`)
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"target out of range", "start at 1", "points_milli", "needs streak_rewards", "only for incense"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}
}

var d0 = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

func day(n int) time.Time { return d0.AddDate(0, 0, n) }

func TestStreakRules(t *testing.T) {
	for _, tc := range []struct {
		name       string
		last       time.Time
		prev       int32
		today      time.Time
		next, live int32
	}{
		{"first ever", time.Time{}, 0, day(0), 1, 0},
		{"consecutive", day(0), 1, day(1), 2, 1},
		{"long run", day(9), 9, day(10), 10, 9},
		{"missed one day", day(0), 5, day(2), 1, 0},
		{"same day (live)", day(3), 4, day(3), 1, 4},
		{"clock skew back", day(5), 3, day(4), 1, 0},
	} {
		if got := NextStreak(tc.last, tc.prev, tc.today); got != tc.next {
			t.Errorf("%s: next %d want %d", tc.name, got, tc.next)
		}
		if got := LiveStreak(tc.last, tc.prev, tc.today); got != tc.live {
			t.Errorf("%s: live %d want %d", tc.name, got, tc.live)
		}
	}
}

func TestStreakPointsAndView(t *testing.T) {
	task := &Task{Kind: Incense, Target: 1, StreakRewards: []StreakReward{{1, 1000}, {3, 1500}, {7, 2000}}}
	for streak, want := range map[int32]int64{0: 0, 1: 1000, 2: 1000, 3: 1500, 6: 1500, 7: 2000, 100: 2000} {
		if got := task.StreakPoints(streak); got != want {
			t.Errorf("streak %d: %d want %d", streak, got, want)
		}
	}
	now := day(3).Add(5 * time.Hour) // 12:00 ICT of day 3
	row := &db.RitualStreak{LastDay: pgtype.Date{Time: day(2), Valid: true}, Streak: 2, Best: 4}
	v := StreakOf(task, row, now)
	if v.Current != 2 || v.OfferedToday || v.NextPointsMilli != 1500 || v.Best != 4 {
		t.Fatalf("yesterday: %+v", v)
	}
	row.LastDay.Time, row.Streak = day(3), 3
	if v := StreakOf(task, row, now); !v.OfferedToday || v.Current != 3 || v.NextPointsMilli != 1500 {
		t.Fatalf("today: %+v", v)
	}
	row.LastDay.Time = day(0)
	if v := StreakOf(task, row, now); v.Current != 0 || v.NextPointsMilli != 1000 {
		t.Fatalf("broken: %+v", v)
	}
	if v := StreakOf(task, nil, now); v.Current != 0 || v.Best != 0 || v.NextPointsMilli != 1000 {
		t.Fatalf("never: %+v", v)
	}
}
