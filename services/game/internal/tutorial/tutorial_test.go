package tutorial

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"tiendao/game/internal/realm"
)

const storyDir = "../../../../content/story/tutorial"

func TestShippedTutorial(t *testing.T) {
	realms, err := realm.LoadFile("../../../../content/catalog/realms.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Load(storyDir, Vars(realms))
	if err != nil {
		t.Fatal(err)
	}
	if s.Guide != "van_khe" || s.Name != "Vân Khê" {
		t.Fatalf("guide = %s %s (decisions 2026-11-09 #1)", s.Guide, s.Name)
	}
	// Brief §4C: onboarding + every "lần đầu".
	for _, id := range []string{"ob_tu_tieu_cung", "ob_the_luc", "ob_ngoai_hinh", "ob_dao_hieu", "welcome", "first_settle",
		"first_stage_up", "first_breakthrough", "first_realm", "first_tribulation", "first_faction", "first_lecture",
		"first_shop", "first_minigame", "first_tien_duyen", "first_dao_lu"} {
		if _, ok := s.byID[id]; !ok {
			t.Errorf("missing step %q", id)
		}
	}
	tr := realms.Tribulation()
	trib := s.byID["first_tribulation"]
	joined := ""
	for _, l := range trib.Lines {
		joined += l.Text + " "
	}
	// The tribulation numbers come from realms.yaml, never typed by hand.
	for _, want := range []string{Vars(realms)["loss_pct"] + "%", Vars(realms)["guarantee_after"] + " lần"} {
		if !strings.Contains(joined, want) {
			t.Errorf("first_tribulation must state %q (loss_bp %d, guarantee_after %d): %s", want, tr.LossBP, tr.GuaranteeAfter, joined)
		}
	}
	for _, st := range s.Steps {
		for _, l := range st.Lines {
			if strings.Contains(l.Text, "{") {
				t.Errorf("%s: unsubstituted placeholder: %s", st.ID, l.Text)
			}
			if utf8.RuneCountInString(l.Text) > MaxLineRunes {
				t.Errorf("%s: line too long", st.ID)
			}
		}
	}
	// Tiên Duyên is never framed as power (CLAUDE.md non-negotiable 3).
	for _, l := range s.byID["first_tien_duyen"].Lines {
		if strings.Contains(l.Text, "mạnh hơn") {
			t.Errorf("first_tien_duyen frames Tiên Duyên as power: %s", l.Text)
		}
	}
	if !s.Stored("first_settle") || s.Stored("ob_the_luc") || s.Stored("nope") {
		t.Fatal("Stored: play steps only")
	}
}

func writeScript(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

const okScript = `version: 1
guide: van_khe
name: Vân Khê
steps:
  - id: first_settle
    flow: play
    trigger: fx.settled
    title: T
    lines:
      - { expr: smile, text: "Mất {loss_pct}% tu vi." }
`

func TestLoadRejects(t *testing.T) {
	vars := map[string]string{"loss_pct": "25"}
	if s, err := Load(writeScript(t, okScript), vars); err != nil || s.Steps[0].Lines[0].Text != "Mất 25% tu vi." {
		t.Fatalf("ok: %v", err)
	}
	long := strings.Repeat("a", MaxLineRunes+1)
	for name, tc := range map[string]struct{ from, to, want string }{
		"too long":        {`text: "Mất {loss_pct}% tu vi."`, `text: "` + long + `"`, "characters"},
		"bad expr":        {"expr: smile", "expr: angry", "unknown expr"},
		"bad trigger":     {"trigger: fx.settled", "trigger: fx.reward", "unknown trigger"},
		"flow mismatch":   {"trigger: fx.settled", "trigger: onboarding.the_luc", "onboarding triggers"},
		"placeholder":     {"{loss_pct}", "{odds}", "unknown placeholder"},
		"unknown field":   {"title: T", "title: T\n    reward: 5", "field reward not found"},
		"bad id":          {"id: first_settle", "id: First", "invalid id"},
		"bad spotlight":   {"expr: smile,", "expr: smile, spotlight: Qi Bar,", "invalid spotlight"},
		"missing lines":   {"    lines:\n      - { expr: smile, text: \"Mất {loss_pct}% tu vi.\" }\n", "    lines: []\n", "title and lines"},
		"another guide":   {"guide: van_khe", "guide: hac_thu", ""},
		"wrong version":   {"version: 1", "version: 2", "unsupported version"},
		"empty text line": {`text: "Mất {loss_pct}% tu vi."`, `text: ""`, "characters"},
	} {
		t.Run(name, func(t *testing.T) {
			src := strings.Replace(okScript, tc.from, tc.to, 1)
			if src == okScript {
				t.Fatalf("replacement %q not found", tc.from)
			}
			_, err := Load(writeScript(t, src), vars)
			if tc.want == "" {
				return // single file: another guide name is just the guide
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
	// Two files must agree on the guide; a trigger belongs to one step.
	dir := writeScript(t, okScript)
	_ = os.WriteFile(filepath.Join(dir, "b.yaml"), []byte(strings.Replace(okScript, "id: first_settle", "id: again", 1)), 0o644)
	if _, err := Load(dir, vars); err == nil || !strings.Contains(err.Error(), "share trigger") {
		t.Fatalf("shared trigger: %v", err)
	}
}
