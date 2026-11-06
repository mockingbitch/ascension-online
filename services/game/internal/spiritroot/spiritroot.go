// Package spiritroot loads spirit-root definitions (content/catalog/spirit_roots.yaml).
package spiritroot

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// MaxRateBonusBP caps any single root's cultivation bonus (economy-balance rule: ≤ 10%).
const MaxRateBonusBP = 1000

var (
	slugRe   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	elements = map[string]bool{"kim": true, "moc": true, "thuy": true, "hoa": true, "tho": true, "dual": true, "rare": true}
)

// Root is one spirit root.
type Root struct {
	ID          string `yaml:"id" json:"id"`
	Name        string `yaml:"name" json:"name"`
	Element     string `yaml:"element" json:"element"`
	RateBonusBP int64  `yaml:"rate_bonus_bp" json:"rateBonusBp"`
}

// Catalogue is immutable after load.
type Catalogue struct {
	roots []Root
	byID  map[string]*Root
}

type file struct {
	Version int    `yaml:"version"`
	Roots   []Root `yaml:"roots"`
}

// LoadFile parses and validates spirit_roots.yaml.
func LoadFile(path string) (*Catalogue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var raw file
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("spiritroot: decode: %w", err)
	}
	var errs []error
	c := &Catalogue{roots: raw.Roots, byID: map[string]*Root{}}
	for i := range c.roots {
		r := &c.roots[i]
		if !slugRe.MatchString(r.ID) {
			errs = append(errs, fmt.Errorf("root[%d]: invalid id %q", i, r.ID))
		}
		if _, dup := c.byID[r.ID]; dup {
			errs = append(errs, fmt.Errorf("root %q: duplicate", r.ID))
		}
		if !elements[r.Element] {
			errs = append(errs, fmt.Errorf("root %q: unknown element %q", r.ID, r.Element))
		}
		if r.RateBonusBP < 0 || r.RateBonusBP > MaxRateBonusBP {
			errs = append(errs, fmt.Errorf("root %q: rate_bonus_bp must be 0..%d", r.ID, MaxRateBonusBP))
		}
		c.byID[r.ID] = r
	}
	if len(c.roots) == 0 {
		errs = append(errs, errors.New("no roots"))
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("spiritroot: invalid catalogue: %w", errors.Join(errs...))
	}
	return c, nil
}

// Get returns a root by ID.
func (c *Catalogue) Get(id string) (*Root, bool) { r, ok := c.byID[id]; return r, ok }

// All returns roots in file order.
func (c *Catalogue) All() []Root { return c.roots }
