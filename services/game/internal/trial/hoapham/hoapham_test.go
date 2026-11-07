package hoapham

import (
	"errors"
	"testing"
)

// testParams mirrors hoaphamtest.Params (an internal test cannot import that package: import cycle).
func testParams() *Params {
	return &Params{
		RulesVersion: 1, TickMS: 40, Width: 120, Height: 160, TimeLimitTicks: 2250, PassScore: 75, MinPassTicks: 1500,
		Lanterns: 3, GravitySub: 3,
		Knot:       Motion{VY: Range{Min: 125, Max: 170}, VXMax: 12},
		Lantern:    Motion{VY: Range{Min: 40, Max: 60}, VXMax: 6},
		Spawn:      Spawn{StartTicks: 44, MinTicks: 22, StepEveryTicks: 60, WaveMax: 4, WaveGrowEveryTicks: 500, MarginUnits: 16},
		MinCutStep: 3, Combo: Combo{StepBonus: 1, MaxBonus: 3},
		Calm:       Calm{ChargePerCut: 1, ChargeMax: 12, DurationTicks: 125},
		MaxStrokes: 600, MaxStep: 48, InputWindowTicks: 25, MaxWindowTravel: 720, MaxStrokesPerWindow: 8, MinLiftTicks: 2,
		Objects: []Object{
			{ID: "ket", Name: "Kết Trần Duyên", Role: RoleKnot, Weight: 6, Radius: 9, Points: 1, HP: 1},
			{ID: "to", Name: "Tơ Vương", Role: RoleKnot, Weight: 3, Radius: 6, Points: 2, HP: 1},
			{ID: "luoi", Name: "Lưới Nghiệp", Role: RoleKnot, Weight: 1, Radius: 13, Points: 3, HP: 2},
			{ID: "den", Name: "Đăng Chân Tâm", Role: RoleLantern, Weight: 2, Radius: 9, Points: 0, HP: 1},
		},
	}
}

func TestParamsValid(t *testing.T) {
	if err := testParams().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsBadParams(t *testing.T) {
	cases := map[string]func(p *Params){
		"rules_version":     func(p *Params) { p.RulesVersion = 2 },
		"tick_ms":           func(p *Params) { p.TickMS = 5 },
		"tiny field":        func(p *Params) { p.Width = 10 },
		"min pass ≥ time":   func(p *Params) { p.MinPassTicks = p.TimeLimitTicks },
		"no lanterns lives": func(p *Params) { p.Lanterns = 0 },
		"throw out the top": func(p *Params) { p.Knot.VY.Max = 400 },
		"vy range":          func(p *Params) { p.Lantern.VY.Min = 70 },
		"spawn":             func(p *Params) { p.Spawn.MinTicks = 100 },
		"margin":            func(p *Params) { p.Spawn.MarginUnits = 60 },
		"max step < cut":    func(p *Params) { p.MaxStep = 2 },
		"max step huge":     func(p *Params) { p.MaxStep = 100 },
		"window travel":     func(p *Params) { p.MaxWindowTravel = 5000 },
		"window strokes":    func(p *Params) { p.MaxStrokesPerWindow = 40 },
		"window short":      func(p *Params) { p.InputWindowTicks = 5 },
		"lift zero":         func(p *Params) { p.MinLiftTicks = 0 },
		"duplicate id":      func(p *Params) { p.Objects[1].ID = "ket" },
		"bad role":          func(p *Params) { p.Objects[0].Role = "fruit" },
		"lantern points":    func(p *Params) { p.Objects[3].Points = 2 },
		"no lantern":        func(p *Params) { p.Objects = p.Objects[:3] },
		"radius":            func(p *Params) { p.Objects[0].Radius = 50 },
	}
	for name, mut := range cases {
		p := testParams()
		mut(p)
		if p.Validate() == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestXorShiftKnownValues(t *testing.T) {
	x := NewXorShift32(1)
	for i, w := range []uint32{270369, 67634689, 2647435461, 307599695, 2398689233} {
		if got := x.Next(); got != w {
			t.Fatalf("draw %d: got %d want %d", i, got, w)
		}
	}
}

func TestStepLenAndToward(t *testing.T) {
	for _, c := range []struct{ dx, dy, want int64 }{{0, 0, 0}, {3, 0, 3}, {0, -5, 5}, {4, 4, 6}, {-10, 3, 11}, {7, -2, 8}} {
		if got := StepLen(c.dx, c.dy); got != c.want {
			t.Errorf("StepLen(%d,%d) = %d want %d", c.dx, c.dy, got, c.want)
		}
	}
	// Toward never overshoots its budget, in every direction.
	for dx := int64(-90); dx <= 90; dx += 7 {
		for dy := int64(-90); dy <= 90; dy += 11 {
			for _, b := range []int64{0, 1, 5, 48} {
				x, y := Toward(50, 50, 50+dx, 50+dy, b)
				if l := StepLen(x-50, y-50); l > b && l > StepLen(dx, dy) {
					t.Fatalf("Toward(%d,%d,%d): step %d > budget", dx, dy, b, l)
				}
			}
		}
	}
	if x, y := Toward(0, 0, 3, 4, 48); x != 3 || y != 4 {
		t.Fatal("within budget must reach the target")
	}
}

func TestSegHits(t *testing.T) {
	r := int64(9 * Sub)
	cases := []struct {
		name               string
		ax, ay, bx, by, cx int64
		cy                 int64
		want               bool
	}{
		{"through centre", 0, 0, 100 * Sub, 0, 50 * Sub, 0, true},
		{"grazes edge", 0, 0, 100 * Sub, 0, 50 * Sub, 9 * Sub, true},
		{"just misses", 0, 0, 100 * Sub, 0, 50 * Sub, 9*Sub + 1, false},
		{"before start", 0, 0, 100 * Sub, 0, -12 * Sub, 0, false},
		{"near start cap", 0, 0, 100 * Sub, 0, -8 * Sub, 0, true},
		{"past end cap", 0, 0, 100 * Sub, 0, 110 * Sub, 0, false},
		{"diagonal", 0, 0, 100 * Sub, 100 * Sub, 50 * Sub, 60 * Sub, true},
	}
	for _, c := range cases {
		if got := segHits(c.ax, c.ay, c.bx, c.by, c.cx, c.cy, r); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

// A knot right on the path of a fast stroke is cut; a slow drag over it cuts nothing; a stroke hits a 2-hp net once.
func TestCutRules(t *testing.T) {
	p := testParams()
	place := func(typ int64, xu, yu int64) *Game {
		g := New(p, 5)
		g.spawnCounter = -1 << 40 // no spawns
		g.objs = []Obj{{ID: 1, Type: typ, X: xu * Sub, Y: yu * Sub, HP: p.Objects[typ].HP}}
		return g
	}
	g := place(0, 60, 80)
	g.Step(true, 40, 80)
	g.Step(true, 70, 80)
	if g.cuts != 1 || g.score != 1 || len(g.objs) != 0 {
		t.Fatalf("fast stroke: cuts %d score %d objs %d", g.cuts, g.score, len(g.objs))
	}
	g = place(0, 60, 80)
	g.Step(true, 58, 80)
	g.Step(true, 60, 80) // 2 units < min_cut_step
	g.Step(true, 62, 80)
	if g.cuts != 0 {
		t.Fatal("a slow drag must not cut")
	}
	g = place(2, 60, 80) // lưới nghiệp, hp 2
	g.Step(true, 40, 80)
	g.Step(true, 70, 80)
	g.Step(true, 40, 80) // same stroke back over it: ignored
	if g.cuts != 0 || len(g.objs) != 1 || g.objs[0].HP != 1 {
		t.Fatalf("net after one stroke: cuts %d objs %+v", g.cuts, g.objs)
	}
	g.objs[0].X, g.objs[0].Y, g.objs[0].VX, g.objs[0].VY = 60*Sub, 80*Sub, 0, 0
	g.Step(false, 0, 0)
	g.objs[0].X, g.objs[0].Y, g.objs[0].VX, g.objs[0].VY = 60*Sub, 80*Sub, 0, 0
	g.Step(true, 40, 80)
	g.objs[0].X, g.objs[0].Y, g.objs[0].VX, g.objs[0].VY = 60*Sub, 80*Sub, 0, 0
	g.Step(true, 70, 80)
	if g.cuts != 1 || g.score != 3 {
		t.Fatalf("net after second stroke: cuts %d score %d", g.cuts, g.score)
	}
}

func TestComboAndLanterns(t *testing.T) {
	p := testParams()
	g := New(p, 5)
	g.spawnCounter = -1 << 40
	g.objs = []Obj{
		{ID: 1, Type: 0, X: 20 * Sub, Y: 80 * Sub, HP: 1},
		{ID: 2, Type: 0, X: 50 * Sub, Y: 80 * Sub, HP: 1},
		{ID: 3, Type: 1, X: 80 * Sub, Y: 80 * Sub, HP: 1},
	}
	g.Step(true, 5, 80)
	g.Step(true, 50, 80)
	g.Step(true, 95, 80)
	// 1 + (1+1) + (2+2) = 7
	if g.cuts != 3 || g.score != 7 || g.bestCombo != 3 {
		t.Fatalf("combo: cuts %d score %d best %d", g.cuts, g.score, g.bestCombo)
	}
	h := New(p, 5)
	h.spawnCounter = -1 << 40
	for i := range p.Lanterns {
		h.objs = append(h.objs, Obj{ID: i + 1, Type: 3, X: (20 + 30*i) * Sub, Y: 80 * Sub, HP: 1})
	}
	h.Step(true, 5, 80)
	h.Step(true, 50, 80)
	if h.over || h.lanternsLeft != 1 {
		t.Fatalf("two lanterns cut: over %v left %d", h.over, h.lanternsLeft)
	}
	h.Step(true, 95, 80)
	if !h.over || h.won || h.reason != ReasonLanternsOut {
		t.Fatalf("all lanterns: %+v", h.Result())
	}
}

func TestCalmSlowsTheWorld(t *testing.T) {
	p := testParams()
	g := New(p, 3)
	g.spawnCounter = -1 << 40
	g.objs = []Obj{{ID: 1, Type: 0, X: 60 * Sub, Y: 80 * Sub, VY: -10, HP: 1}}
	g.calmLeft = 10
	for range 10 {
		g.Step(false, 0, 0)
	}
	if moved := 80*Sub - g.objs[0].Y; moved >= 10*10 {
		t.Fatalf("calm: moved %d sub in 10 ticks, expected about half speed", moved)
	}
	// Charging: charge_max cuts start the calm.
	h := New(p, 3)
	h.charge = p.Calm.ChargeMax - 1
	h.spawnCounter = -1 << 40
	h.objs = []Obj{{ID: 1, Type: 0, X: 60 * Sub, Y: 80 * Sub, HP: 1}}
	h.Step(true, 40, 80)
	h.Step(true, 70, 80)
	if h.calmLeft != p.Calm.DurationTicks-1 || h.charge != 0 {
		t.Fatalf("calm start: left %d charge %d", h.calmLeft, h.charge)
	}
}

func TestMinPassTicks(t *testing.T) {
	p := testParams()
	g := New(p, 9)
	g.spawnCounter = -1 << 40
	g.score = p.PassScore
	for g.tick < p.MinPassTicks-1 {
		g.Step(false, 0, 0)
		if g.over {
			t.Fatalf("ended at %d before min_pass_ticks", g.tick)
		}
	}
	g.Step(false, 0, 0)
	if !g.won || g.reason != ReasonCleared || g.tick != p.MinPassTicks {
		t.Fatalf("pass at min_pass_ticks: %s at %d", g.reason, g.tick)
	}
}

func TestIdleGameTimesOut(t *testing.T) {
	p := testParams()
	r, err := Replay(p, 7, nil, p.TimeLimitTicks)
	if err != nil || r.Reason != ReasonTimeUp || r.Won || r.Cuts != 0 || r.Escaped == 0 || r.Spawned == 0 {
		t.Fatalf("idle: %+v %v", r, err)
	}
}

func TestReplayRejectsMalformedLogs(t *testing.T) {
	p := testParams()
	cases := map[string]struct {
		s   []Stroke
		end int64
	}{
		"zero end":       {nil, 0},
		"past limit":     {nil, p.TimeLimitTicks + 1},
		"short stroke":   {[]Stroke{{1, 2}}, 100},
		"even length":    {[]Stroke{{1, 2, 3, 4}}, 100},
		"negative tick":  {[]Stroke{{-1, 2, 3}}, 100},
		"overlap":        {[]Stroke{{1, 2, 3, 1, 1, 1, 1}, {2, 5, 5}}, 100},
		"out of order":   {[]Stroke{{50, 2, 3}, {10, 5, 5}}, 100},
		"touching":       {[]Stroke{{1, 2, 3, 1, 1}, {3, 5, 5}}, 100},
		"past end":       {[]Stroke{{99, 2, 3, 1, 1}}, 100},
		"start outside":  {[]Stroke{{1, 120, 3}}, 100},
		"leaves field":   {[]Stroke{{1, 110, 3, 20, 0}}, 100},
		"negative coord": {[]Stroke{{1, 2, 3, -5, 0}}, 100},
		"absurd delta":   {[]Stroke{{1, 2, 3, 1 << 40, 0}}, 100},
		// L1: a start near MaxInt64 overflowed End() and slipped past the order/overlap checks.
		"overflow start": {[]Stroke{{10, 5, 5, 1, 1}, {9223372036854775807, 5, 5, 0, 0}, {0, 5, 5, 1, 1}}, 100},
		"start at end":   {[]Stroke{{100, 5, 5}}, 100},
	}
	for name, c := range cases {
		if _, err := Replay(p, 1, c.s, c.end); !errors.Is(err, ErrInvalidLog) {
			t.Errorf("%s: want ErrInvalidLog, got %v", name, err)
		}
	}
	many := make([]Stroke, p.MaxStrokes+1)
	for i := range many {
		many[i] = Stroke{int64(i) * 3, 5, 5}
	}
	if _, err := Replay(p, 1, many, p.TimeLimitTicks); !errors.Is(err, ErrInvalidLog) {
		t.Error("too many strokes accepted")
	}
}

func TestReplayRejectsInhumanLogs(t *testing.T) {
	p := testParams()
	// Teleport: one step of 100 units.
	r, err := Replay(p, 1, []Stroke{{10, 5, 80, 100, 0}}, 100)
	if !errors.Is(err, ErrImplausibleInput) || r.MaxStep != 100 {
		t.Errorf("teleport: %v %+v", err, r)
	}
	// Superhuman sustained speed: max steps back and forth for a whole window.
	fast := Stroke{10, 10, 80}
	for i := range 24 {
		d := p.MaxStep
		if i%2 == 1 {
			d = -d
		}
		fast = append(fast, d, 0)
	}
	if r, err := Replay(p, 1, []Stroke{fast}, 100); !errors.Is(err, ErrImplausibleInput) || r.MaxWindowTravel <= p.MaxWindowTravel {
		t.Errorf("sustained speed: %v %+v", err, r)
	}
	// Machine-gun taps: more stroke starts than the window allows (each gap = min lift).
	var taps []Stroke
	for i := range int64(p.MaxStrokesPerWindow + 1) {
		taps = append(taps, Stroke{i * (p.MinLiftTicks + 1), 5, 5})
	}
	if r, err := Replay(p, 1, taps, 100); !errors.Is(err, ErrImplausibleInput) || r.MaxWindowStrokes != p.MaxStrokesPerWindow+1 {
		t.Errorf("taps: %v %+v", err, r)
	}
	// Re-press after 1 tick when min lift is 2.
	if _, err := Replay(p, 1, []Stroke{{1, 5, 5, 1, 0}, {4, 5, 5}}, 100); !errors.Is(err, ErrImplausibleInput) {
		t.Errorf("fast re-press: %v", err)
	}
	// Exactly at the caps is fine.
	if _, err := Replay(p, 1, []Stroke{{10, 5, 80, p.MaxStep, 0}, {14, 5, 5}}, 100); err != nil {
		t.Errorf("at caps: %v", err)
	}
}
