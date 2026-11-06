package moderation

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func shipped(t *testing.T) *Catalogue {
	t.Helper()
	c, err := LoadFile(filepath.Join("..", "..", "..", "..", "content", "catalog", "moderation.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestShippedModerationIsValid(t *testing.T) {
	c := shipped(t)
	if c.RuleCount() < 50 || c.Reports.DailyLimit != 10 || c.Reports.NoteMaxChars != 500 {
		t.Fatalf("rules=%d reports=%+v", c.RuleCount(), c.Reports)
	}
}

func TestCheckNameShipped(t *testing.T) {
	c := shipped(t)
	blocked := map[string]string{
		"Địt Mẹ":               CategoryProfanity,
		"Dit Me":               CategoryProfanity, // accentless
		"Đ ị t M ẹ":            CategoryProfanity, // spaced out → contains
		"Ditmeee":              CategoryProfanity, // squeezed
		"Lồn Vương":            CategoryProfanity,
		"Lồnnn":                CategoryProfanity,
		"Ｆｕｃｋ":                 CategoryProfanity, // fullwidth (NFKD)
		"Ông Đĩ":               CategoryProfanity,
		"Hoà Đĩ":               CategoryProfanity,
		"Vcl Đạo Nhân":         CategoryProfanity,
		"Admin Lâm":            CategoryImpersonation,
		"Thiên Đạo":            CategoryImpersonation,
		"Ban Quản Trị":         CategoryImpersonation,
		"Hồng Quân":            CategoryStory,
		"Hồng Quân Lão Tổ":     CategoryStory,
		"Nữ Oa Nương Nương":    CategoryStory,
		"Thông Thiên Giáo Chủ": CategoryStory,
		"Quan Âm Bồ Tát":       CategoryReligion,
		"Quán Thế Âm":          CategoryReligion,
		"Hồ Chí Minh":          CategoryPolitical,
	}
	for name, cat := range blocked {
		err := c.CheckName(name)
		var ne *NameError
		if !errors.As(err, &ne) || ne.Category != cat || !errors.Is(err, ErrNameNotAllowed) {
			t.Errorf("CheckName(%q) = %v; want category %s", name, err, cat)
		}
	}
	// False-positive guard: ordinary Vietnamese/xianxia names that share letters with blocked terms.
	for _, ok := range []string{
		"Long Ngạo Thiên", "Côn Lôn Kiếm Khách", "Lôn Ba", "Lý Di", "Di Sơn", "Ngũ Hành Tử", "Nguyễn Hồng Quân",
		"Lâm Thanh Vân", "Mê Mây", "Đỗ Đạt", "Mộ Dung Tuyết", "Thái Nhất Phàm", "Hậu Duệ", "Tiến Đạo",
		"Cao Đề", "Bạch Cổ Lan", "Hàn Lập", "Đông Phương Bất Bại", "Dương Tiễn Phong", "Bắc Ai Long",
	} {
		if err := c.CheckName(ok); err != nil {
			t.Errorf("CheckName(%q) blocked: %+v", ok, err)
		}
	}
}

func TestLoadRejectsBadRules(t *testing.T) {
	const tmpl = "version: 1\nnames:\n  rules:\n%s\nreports:\n  daily_limit: 10\n  note_max_chars: 500\n"
	bad := map[string]string{
		"unknown match":       `    - { term: "abc", match: regex, category: profanity }`,
		"unknown category":    `    - { term: "abcd", match: word, category: rude }`,
		"exact without marks": `    - { term: "long", match: word_exact, category: profanity }`,
		"short contains":      `    - { term: "lon", match: contains, category: profanity }`,
		"digits":              `    - { term: "d1t", match: word, category: profanity }`,
		"duplicate":           "    - { term: \"Dit Me\", match: word, category: profanity }\n    - { term: \"dịt mẹ\", match: word, category: profanity }",
		"empty":               `    - { term: "  ", match: word, category: profanity }`,
		"unknown field":       `    - { term: "abcd", match: word, category: profanity, severity: 3 }`,
	}
	for name, rules := range bad {
		if _, err := Load(strings.NewReader(strings.Replace(tmpl, "%s", rules, 1))); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := Load(strings.NewReader("version: 1\nnames:\n  rules:\n    - { term: \"abcd\", match: word, category: profanity }\nreports:\n  daily_limit: 0\n  note_max_chars: 500\n")); err == nil {
		t.Error("daily_limit 0 accepted")
	}
}

func TestNilCatalogueAllows(t *testing.T) {
	var c *Catalogue
	if c.CheckName("anything") != nil {
		t.Fatal("nil catalogue must allow")
	}
}

func TestMaskTextShipped(t *testing.T) {
	c := shipped(t)
	prof := []string{CategoryProfanity}
	cases := map[string]string{
		"chào đạo hữu":               "chào đạo hữu",
		"địt mẹ mày":                 "***", // "mẹ mày" is a rule too
		"Dit me, ai đột phá chưa?":   "***, ai đột phá chưa?",
		"đ.ị.t m.ẹ thằng kia":        "*** thằng kia",
		"vcl luôn":                   "*** luôn",
		"lồnnn":                      "***",
		"Côn Lôn sơn đẹp quá":        "Côn Lôn sơn đẹp quá",
		"Long ca ơi, lôn ba tầng 3!": "Long ca ơi, lôn ba tầng 3!",
		"ai fuuuck vậy":              "ai *** vậy",
		"Hồng Quân giảng đạo":        "Hồng Quân giảng đạo", // story names are fine in chat
		"admin ơi":                   "admin ơi",            // impersonation is a name rule, not a chat rule
	}
	for in, want := range cases {
		if got, _ := c.MaskText(in, prof); got != want {
			t.Errorf("MaskText(%q) = %q; want %q", in, got, want)
		}
	}
	if _, n := c.MaskText("địt mẹ", nil); n != 0 {
		t.Error("no categories → no masking")
	}
}
