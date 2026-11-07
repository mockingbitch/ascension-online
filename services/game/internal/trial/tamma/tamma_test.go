package tamma

import (
	"errors"
	"slices"
	"testing"
)

// testParams mirrors tammatest.Params (an internal test cannot import that package: import cycle).
func testParams() *Params {
	return &Params{
		RulesVersion: 2, TickMS: 50, Width: 9, Height: 16, TimeLimitTicks: 3600, PassLines: 12,
		Gravity: Gravity{StartTicks: 14, MinTicks: 5, StepEveryTicks: 400}, RiseEveryTicks: 400,
		Calm: Calm{ChargePerLine: 1, ChargeMax: 4, DurationTicks: 160}, LineScores: []int64{0, 100, 300, 600, 1000, 1500},
		MaxInputs: 4000, InputWindowTicks: 20, MaxInputsPerWindow: 16, MinLockGapTicks: 6, MinPassTicks: 1500,
		Pieces: []Piece{
			{ID: "vong", Name: "Vọng Niệm", Weight: 3, Cells: [][2]int{{0, 0}, {1, 0}, {2, 0}}},
			{ID: "chap", Name: "Chấp Niệm", Weight: 3, Cells: [][2]int{{0, 0}, {0, 1}, {1, 1}}},
			{ID: "tham", Name: "Tham", Weight: 1, Cells: [][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 2}, {2, 2}}},
			{ID: "san", Name: "Sân", Weight: 1, Cells: [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}, {0, 2}}},
			{ID: "si", Name: "Si", Weight: 1, Cells: [][2]int{{0, 0}, {1, 0}, {2, 0}, {1, 1}, {1, 2}}},
			{ID: "man", Name: "Mạn", Weight: 1, Cells: [][2]int{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}}},
		},
	}
}

func TestShippedParamsValid(t *testing.T) {
	if err := testParams().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsBadParams(t *testing.T) {
	cases := map[string]func(p *Params){
		"tetromino":       func(p *Params) { p.Pieces[0].Cells = [][2]int{{0, 0}, {1, 0}, {2, 0}, {3, 0}} },
		"disconnected":    func(p *Params) { p.Pieces[0].Cells = [][2]int{{0, 0}, {2, 0}, {4, 0}} },
		"unnormalized":    func(p *Params) { p.Pieces[0].Cells = [][2]int{{1, 0}, {2, 0}, {3, 0}} },
		"duplicate id":    func(p *Params) { p.Pieces[1].ID = "vong" },
		"zero weight":     func(p *Params) { p.Pieces[0].Weight = 0 },
		"tiny board":      func(p *Params) { p.Width = 4 },
		"gravity":         func(p *Params) { p.Gravity.MinTicks = 20 },
		"line_scores":     func(p *Params) { p.LineScores = []int64{0, 300, 100} },
		"rules_version 1": func(p *Params) { p.RulesVersion = 1 },
		"tick_ms":         func(p *Params) { p.TickMS = 5 },
		"window too fast": func(p *Params) { p.MaxInputsPerWindow = 30 }, // 30 per second
		"window short":    func(p *Params) { p.InputWindowTicks = 5 },
		"lock gap zero":   func(p *Params) { p.MinLockGapTicks = 0 },
		"min pass ≥ time": func(p *Params) { p.MinPassTicks = p.TimeLimitTicks },
	}
	for name, mut := range cases {
		p := testParams()
		mut(p)
		if p.Validate() == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestRotationsAreClosed(t *testing.T) {
	g := New(testParams(), 1)
	for i, rs := range g.shapes {
		back := rotateCW(rs[3])
		if !slices.Equal(back.cells, rs[0].cells) {
			t.Errorf("piece %d: four CW rotations do not return to the start: %v vs %v", i, back.cells, rs[0].cells)
		}
	}
}

func TestXorShiftKnownValues(t *testing.T) {
	// Reference values (Marsaglia xorshift32 13/17/5 from seed 1), also asserted in sim.test.ts.
	x := NewXorShift32(1)
	for i, w := range []uint32{270369, 67634689, 2647435461, 307599695, 2398689233} {
		if got := x.Next(); got != w {
			t.Fatalf("draw %d: got %d want %d", i, got, w)
		}
	}
	if NewXorShift32(0).S == 0 {
		t.Fatal("zero seed must be replaced")
	}
}

func TestBagIsAPermutationOfWeights(t *testing.T) {
	p := testParams()
	g := New(p, 42)
	counts := make([]int, len(p.Pieces))
	total := 0
	for _, pc := range p.Pieces {
		total += pc.Weight
	}
	counts[g.bag[0]]++
	counts[g.bag[1]]++
	for range total - 2 {
		counts[g.draw()]++
	}
	for i, pc := range p.Pieces {
		if counts[i] != pc.Weight {
			t.Fatalf("piece %s: %d in bag, want %d", pc.ID, counts[i], pc.Weight)
		}
	}
}

func TestIdleGameTopsOut(t *testing.T) {
	p := testParams()
	g := New(p, 7)
	for !g.Over() {
		g.Step(nil)
	}
	if g.reason != ReasonToppedOut || g.won {
		t.Fatalf("idle game should top out, got %s at %d", g.reason, g.tick)
	}
	r, _ := Replay(p, 7, nil, g.tick)
	if r.Reason != ReasonToppedOut || r.EndTick != g.tick || r.StateHash != g.Hash() {
		t.Fatalf("replay to the real end: %+v", r)
	}
	if r2, _ := Replay(p, 7, nil, p.TimeLimitTicks); r2.Reason != ReasonEndedEarl {
		t.Fatalf("claiming the full limit: %+v", r2)
	}
}

func TestCalmFreezesGravity(t *testing.T) {
	p := testParams()
	g := New(p, 3)
	g.charge = p.Calm.ChargeMax
	g.Step([]Action{ActCalm})
	y := g.py
	for range 100 {
		g.Step(nil)
	}
	if g.py != y || g.charge != 0 || g.calmLeft != p.Calm.DurationTicks-101 {
		t.Fatalf("calm: py %d→%d charge %d left %d", y, g.py, g.charge, g.calmLeft)
	}
	g3 := New(p, 3)
	g3.Step([]Action{ActCalm})
	if g3.calmLeft != 0 {
		t.Fatal("calm without charge")
	}
}

// v2: a hard drop (or a locking soft drop) sooner than min_lock_gap_ticks after the previous lock does nothing.
func TestMinLockGap(t *testing.T) {
	p := testParams()
	g := New(p, 5)
	g.Step([]Action{ActHardDrop}) // tick 0: allowed (lastLock starts at −gap)
	if g.locked != 1 {
		t.Fatalf("first drop: locked %d", g.locked)
	}
	py := g.py
	g.Step([]Action{ActHardDrop}) // tick 1: too soon
	if g.locked != 1 || g.py != py {
		t.Fatalf("drop inside the gap must be ignored: locked %d py %d→%d", g.locked, py, g.py)
	}
	for g.tick < p.MinLockGapTicks {
		g.Step(nil)
	}
	g.Step([]Action{ActHardDrop})
	if g.locked != 2 || g.minGap != p.MinLockGapTicks {
		t.Fatalf("drop after the gap: locked %d minGap %d", g.locked, g.minGap)
	}
}

// v2: reaching pass_lines early does not end the game before min_pass_ticks.
func TestMinPassTicks(t *testing.T) {
	p := testParams()
	g := New(p, 9)
	g.lines = p.PassLines
	for g.tick < p.MinPassTicks-1 {
		g.calmLeft = 1 // keep everything frozen
		g.Step(nil)
		if g.over {
			t.Fatalf("ended at tick %d (%s) before min_pass_ticks", g.tick, g.reason)
		}
	}
	g.calmLeft = 1
	g.Step(nil)
	if !g.won || g.reason != ReasonCleared || g.tick != p.MinPassTicks {
		t.Fatalf("pass at min_pass_ticks: %+v tick %d", g.reason, g.tick)
	}
}

func TestReplayRejectsMalformedAndFastLogs(t *testing.T) {
	p := testParams()
	cases := map[string]struct {
		in  []Input
		end int64
	}{
		"zero end":       {nil, 0},
		"past limit":     {nil, p.TimeLimitTicks + 1},
		"out of order":   {[]Input{{5, ActLeft}, {4, ActLeft}}, 100},
		"at end tick":    {[]Input{{100, ActLeft}}, 100},
		"negative tick":  {[]Input{{-1, ActLeft}}, 100},
		"unknown action": {[]Input{{1, 9}}, 100},
		"zero action":    {[]Input{{1, 0}}, 100},
	}
	for name, c := range cases {
		if _, err := Replay(p, 1, c.in, c.end); !errors.Is(err, ErrInvalidLog) {
			t.Errorf("%s: want ErrInvalidLog, got %v", name, err)
		}
	}
	many := make([]Input, p.MaxInputs+1)
	for i := range many {
		many[i] = Input{Tick: int64(i), Action: ActLeft}
	}
	if _, err := Replay(p, 1, many, p.TimeLimitTicks); !errors.Is(err, ErrInvalidLog) {
		t.Errorf("too many inputs accepted")
	}
	// 17 inputs inside 20 ticks: superhuman cadence.
	burst := make([]Input, p.MaxInputsPerWindow+1)
	for i := range burst {
		burst[i] = Input{Tick: int64(i), Action: ActLeft}
	}
	r, err := Replay(p, 1, burst, 100)
	if !errors.Is(err, ErrImplausibleInput) || r.MaxWindowInputs != p.MaxInputsPerWindow+1 {
		t.Errorf("burst: %v %+v", err, r)
	}
	// Exactly the cap is fine; the same count spread over 21 ticks too.
	if _, err := Replay(p, 1, burst[:p.MaxInputsPerWindow], 100); err != nil {
		t.Errorf("at cap: %v", err)
	}
}

func TestMaxWindowInputs(t *testing.T) {
	in := func(ticks ...int64) []Input {
		out := make([]Input, len(ticks))
		for i, t := range ticks {
			out[i] = Input{Tick: t, Action: ActLeft}
		}
		return out
	}
	cases := []struct {
		ticks []int64
		want  int
	}{
		{nil, 0},
		{[]int64{0}, 1},
		{[]int64{0, 0, 0}, 3},
		{[]int64{0, 19, 20, 39}, 2},
		{[]int64{0, 5, 10, 15, 19, 40}, 5},
	}
	for _, c := range cases {
		if got := MaxWindowInputs(in(c.ticks...), 20); got != c.want {
			t.Errorf("%v: %d want %d", c.ticks, got, c.want)
		}
	}
}
