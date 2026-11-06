// Package moderation holds the name filter and report settings (content/catalog/moderation.yaml).
// Pure: no I/O beyond loading. Report transactions live in internal/report.
package moderation

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	"gopkg.in/yaml.v3"
)

// ErrNameNotAllowed matches every *NameError via errors.Is.
var ErrNameNotAllowed = errors.New("moderation: name not allowed")

// Name rule categories (pick the player-facing message).
const (
	CategoryProfanity     = "profanity"
	CategoryImpersonation = "impersonation"
	CategoryStory         = "story"
	CategoryReligion      = "religion"
	CategoryPolitical     = "political"
)

// NameError says which category blocked a name. The matched term is for logs/admin only — never send it to players.
type NameError struct {
	Category string
	Term     string
}

func (e *NameError) Error() string        { return fmt.Sprintf("moderation: name blocked (%s)", e.Category) }
func (e *NameError) Is(target error) bool { return target == ErrNameNotAllowed }

type matchMode string

const (
	matchWord      matchMode = "word"
	matchWordExact matchMode = "word_exact"
	matchNameExact matchMode = "name_exact"
	matchContains  matchMode = "contains"
)

// ReportSettings is the reports block of moderation.yaml.
type ReportSettings struct {
	DailyLimit   int // reports per reporter per game day
	NoteMaxChars int // max runes of the optional note
}

// Catalogue is the validated, immutable moderation config.
type Catalogue struct {
	rules   []rule
	Reports ReportSettings
}

type rule struct {
	term     string
	mode     matchMode
	category string
	form     form
}

// form is a name/term normalised for matching.
type form struct {
	folded []string // per word: accent-free letters, squeezed ("lồnnn" → "lon")
	exact  []string // per word: folded letters + sorted unique marks (tone placement independent; đ kept as a mark)
	joined string   // all folded letters without spaces, squeezed
}

// latinFold maps Latin letters that NFKD does not decompose to their plain look-alike.
var latinFold = map[rune]string{
	'đ': "d", 'ð': "d", 'ø': "o", 'ł': "l", 'ß': "ss", 'æ': "ae", 'œ': "oe", 'ı': "i", 'ſ': "s",
	'ħ': "h", 'ŧ': "t", 'ƀ': "b", 'ɨ': "i", 'ʉ': "u", 'ƚ': "l", 'ȼ': "c", 'ɇ': "e", 'ɍ': "r", 'ɏ': "y",
}

const dMark = 'đ' // stored among the marks so that word_exact distinguishes đ from d

func normalise(s string) form {
	var f form
	var all strings.Builder
	for _, w := range strings.Fields(norm.NFKD.String(strings.ToLower(s))) {
		var letters strings.Builder
		var marks []rune
		for _, r := range w {
			switch {
			case unicode.Is(unicode.Mn, r):
				marks = append(marks, r)
			case r == 'đ':
				letters.WriteByte('d')
				marks = append(marks, dMark)
			default:
				if m, ok := latinFold[r]; ok {
					letters.WriteString(m)
				} else {
					letters.WriteRune(unicode.ToLower(r))
				}
			}
		}
		l := squeeze(letters.String())
		slices.Sort(marks)
		marks = slices.Compact(marks)
		f.folded = append(f.folded, l)
		f.exact = append(f.exact, l+"|"+string(marks))
		all.WriteString(letters.String())
	}
	f.joined = squeeze(all.String())
	return f
}

// squeeze collapses runs of the same rune ("fuuuck" → "fuck").
func squeeze(s string) string {
	var b strings.Builder
	var prev rune = -1
	for _, r := range s {
		if r != prev {
			b.WriteRune(r)
		}
		prev = r
	}
	return b.String()
}

func containsSeq(hay, needle []string) bool {
	if len(needle) == 0 || len(needle) > len(hay) {
		return false
	}
	for i := 0; i+len(needle) <= len(hay); i++ {
		if slices.Equal(hay[i:i+len(needle)], needle) {
			return true
		}
	}
	return false
}

func (r rule) matches(n form) bool {
	switch r.mode {
	case matchWord:
		return containsSeq(n.folded, r.form.folded)
	case matchWordExact:
		return containsSeq(n.exact, r.form.exact)
	case matchNameExact:
		return slices.Equal(n.exact, r.form.exact)
	case matchContains:
		return strings.Contains(n.joined, r.form.joined)
	}
	return false
}

// CheckName returns a *NameError when an (already format-normalised) name hits a rule. Nil catalogue allows all.
func (c *Catalogue) CheckName(name string) error {
	if c == nil {
		return nil
	}
	n := normalise(name)
	for _, r := range c.rules {
		if r.matches(n) {
			return &NameError{Category: r.category, Term: r.term}
		}
	}
	return nil
}

// RuleCount is the number of loaded name rules.
func (c *Catalogue) RuleCount() int { return len(c.rules) }

// ───── loader ─────

type file struct {
	Version int `yaml:"version"`
	Names   struct {
		Rules []struct {
			Term     string `yaml:"term"`
			Match    string `yaml:"match"`
			Category string `yaml:"category"`
		} `yaml:"rules"`
	} `yaml:"names"`
	Reports struct {
		DailyLimit   int `yaml:"daily_limit"`
		NoteMaxChars int `yaml:"note_max_chars"`
	} `yaml:"reports"`
}

// LoadFile parses and validates moderation.yaml.
func LoadFile(path string) (*Catalogue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load parses and validates moderation YAML, collecting every error.
func Load(r io.Reader) (*Catalogue, error) {
	var raw file
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("moderation: decode: %w", err)
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if raw.Version != 1 {
		fail("unsupported version %d", raw.Version)
	}
	if len(raw.Names.Rules) == 0 || len(raw.Names.Rules) > 5000 {
		fail("names.rules: need 1..5000 rules, got %d", len(raw.Names.Rules))
	}
	c := &Catalogue{Reports: ReportSettings{DailyLimit: raw.Reports.DailyLimit, NoteMaxChars: raw.Reports.NoteMaxChars}}
	seen := map[string]bool{}
	for i, rr := range raw.Names.Rules {
		at := fmt.Sprintf("names.rules[%d] %q", i, rr.Term)
		mode := matchMode(rr.Match)
		switch mode {
		case matchWord, matchWordExact, matchNameExact, matchContains:
		default:
			fail("%s: match must be word|word_exact|name_exact|contains", at)
			continue
		}
		switch rr.Category {
		case CategoryProfanity, CategoryImpersonation, CategoryStory, CategoryReligion, CategoryPolitical:
		default:
			fail("%s: unknown category %q", at, rr.Category)
		}
		term := strings.Join(strings.Fields(norm.NFC.String(rr.Term)), " ")
		if term == "" {
			fail("%s: empty term", at)
			continue
		}
		if strings.IndexFunc(term, func(r rune) bool { return !(r == ' ' || unicode.IsLetter(r) || unicode.Is(unicode.Mn, r)) }) >= 0 {
			fail("%s: term may contain only letters and spaces (names cannot contain anything else)", at)
			continue
		}
		f := normalise(term)
		switch mode {
		case matchWordExact, matchNameExact:
			if !hasMarks(f) {
				fail("%s: %s needs a diacritic (without one, use match: word)", at, mode)
			}
		case matchContains:
			if utf8.RuneCountInString(f.joined) < 4 {
				fail("%s: contains terms need ≥ 4 letters (shorter ones block innocent names)", at)
			}
		}
		k := string(mode) + ":" + strings.Join(f.exact, " ")
		if mode == matchWord || mode == matchContains {
			k = string(mode) + ":" + strings.Join(f.folded, " ")
		}
		if seen[k] {
			fail("%s: duplicate rule", at)
		}
		seen[k] = true
		c.rules = append(c.rules, rule{term: term, mode: mode, category: rr.Category, form: f})
	}
	if c.Reports.DailyLimit < 1 || c.Reports.DailyLimit > 100 {
		fail("reports.daily_limit must be within 1..100")
	}
	if c.Reports.NoteMaxChars < 0 || c.Reports.NoteMaxChars > 1000 {
		fail("reports.note_max_chars must be within 0..1000 (DB limit)")
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("moderation: invalid catalogue: %w", errors.Join(errs...))
	}
	return c, nil
}

func hasMarks(f form) bool {
	for _, e := range f.exact {
		if !strings.HasSuffix(e, "|") {
			return true
		}
	}
	return false
}

// ───── chat text masking ─────

// textToken is one run of letters (with their combining marks) in the original text.
type textToken struct {
	start, end int  // byte span in the original string
	f          form // normalised form of this single word
}

// HasCategory reports whether any rule uses category (used by the chat loader to validate mask_categories).
func (c *Catalogue) HasCategory(category string) bool {
	for _, r := range c.rules {
		if r.category == category {
			return true
		}
	}
	return false
}

// MaskText replaces every span hit by a rule of the given categories with "***" and returns the masked text and
// the number of hits. Words are runs of letters; anything else (digits, punctuation, spaces) separates words, so
// "đ.ị.t m.ẹ" is caught by `contains` rules and masked as a whole. name_exact rules never apply to free text.
func (c *Catalogue) MaskText(text string, categories []string) (string, int) {
	if c == nil || text == "" {
		return text, 0
	}
	src := norm.NFC.String(text)
	var toks []textToken
	start := -1
	flush := func(end int) {
		if start >= 0 {
			toks = append(toks, textToken{start: start, end: end, f: normalise(src[start:end])})
			start = -1
		}
	}
	for i, r := range src {
		if unicode.IsLetter(r) || unicode.Is(unicode.Mn, r) {
			if start < 0 {
				start = i
			}
		} else {
			flush(i)
		}
	}
	flush(len(src))
	if len(toks) == 0 {
		return src, 0
	}

	folded := make([]string, len(toks))
	exact := make([]string, len(toks))
	// joined: squeezed folded letters of all words; owner[i] = [first, last] token covering squeezed rune i.
	var joined []rune
	var owner [][2]int
	for i, t := range toks {
		folded[i], exact[i] = t.f.folded[0], t.f.exact[0]
		for _, r := range t.f.joined {
			if n := len(joined); n > 0 && joined[n-1] == r {
				owner[n-1][1] = i
				continue
			}
			joined = append(joined, r)
			owner = append(owner, [2]int{i, i})
		}
	}
	hit := make([]bool, len(toks))
	hits := 0
	for _, r := range c.rules {
		if !slices.Contains(categories, r.category) {
			continue
		}
		switch r.mode {
		case matchWord, matchWordExact:
			hay, needle := folded, r.form.folded
			if r.mode == matchWordExact {
				hay, needle = exact, r.form.exact
			}
			for i := 0; i+len(needle) <= len(hay); i++ {
				if slices.Equal(hay[i:i+len(needle)], needle) {
					hits++
					for k := i; k < i+len(needle); k++ {
						hit[k] = true
					}
				}
			}
		case matchContains:
			needle := []rune(r.form.joined)
			for i := 0; i+len(needle) <= len(joined); i++ {
				if slices.Equal(joined[i:i+len(needle)], needle) {
					hits++
					for k := owner[i][0]; k <= owner[i+len(needle)-1][1]; k++ {
						hit[k] = true
					}
				}
			}
		}
	}
	if hits == 0 {
		return src, 0
	}
	// Rebuild: a masked run of words (and the separators between them) becomes a single "***".
	var b strings.Builder
	prev := 0
	for i := 0; i < len(toks); i++ {
		if !hit[i] {
			continue
		}
		j := i
		for j+1 < len(toks) && hit[j+1] {
			j++
		}
		b.WriteString(src[prev:toks[i].start])
		b.WriteString("***")
		prev = toks[j].end
		i = j
	}
	b.WriteString(src[prev:])
	return b.String(), hits
}
