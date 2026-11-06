// Package character handles character creation and the character view.
package character

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// ErrInvalidName is returned for names outside the allowed format.
var ErrInvalidName = errors.New("character: name must be 2–24 Latin/Vietnamese letters and single spaces")

// NormalizeName NFC-normalizes, trims and collapses spaces, then validates:
// Latin-script letters (covers Vietnamese, incl. đ/ơ/ư and tone marks) and single spaces, 2–24 runes.
// Other scripts are rejected to block invisible fillers (U+3164…) and lookalikes (Cyrillic "А").
// Combining marks are allowed only directly after a letter or another mark.
func NormalizeName(raw string) (string, error) {
	s := strings.Join(strings.Fields(norm.NFC.String(raw)), " ")
	n := utf8.RuneCountInString(s)
	if n < 2 || n > 24 {
		return "", ErrInvalidName
	}
	prevLetter := false
	for _, r := range s {
		switch {
		case r == ' ':
			prevLetter = false
		case unicode.Is(unicode.Latin, r) && unicode.IsLetter(r):
			prevLetter = true
		case unicode.Is(unicode.Mn, r) && prevLetter:
		default:
			return "", ErrInvalidName
		}
	}
	return s, nil
}
