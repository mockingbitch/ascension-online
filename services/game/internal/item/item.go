// Package item is the immutable item-definition catalogue from content/catalog/items.yaml.
package item

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Category groups items for UI and rules.
type Category string

// Rarity maps to design tokens --color-rarity-<rarity>.
type Rarity string

var (
	categories = map[Category]bool{"pill": true, "herb": true, "material": true, "fragment": true, "consumable": true, "currency_bundle": true, "cosmetic": true}
	rarities   = map[Rarity]bool{"pham": true, "linh": true, "huyen": true, "dia": true, "thien": true, "tien": true}
	slugRe     = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// Def is one item definition.
type Def struct {
	ID          string   `yaml:"id" json:"id"`
	Name        string   `yaml:"name" json:"name"`
	Category    Category `yaml:"category" json:"category"`
	Rarity      Rarity   `yaml:"rarity" json:"rarity"`
	Tradable    bool     `yaml:"tradable" json:"tradable"`
	Description string   `yaml:"description" json:"description"`
	// Effect parameters (optional, category-specific).
	OfflineCapBonusHours int `yaml:"offline_cap_bonus_hours,omitempty" json:"offlineCapBonusHours,omitempty"`
}

// Catalogue is immutable after load.
type Catalogue struct {
	items []Def
	byID  map[string]*Def
}

type file struct {
	Version int   `yaml:"version"`
	Items   []Def `yaml:"items"`
}

// LoadFile parses and validates items.yaml.
func LoadFile(path string) (*Catalogue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load parses and validates item YAML.
func Load(r io.Reader) (*Catalogue, error) {
	var raw file
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("item: decode: %w", err)
	}
	var errs []error
	if raw.Version != 1 {
		errs = append(errs, fmt.Errorf("unsupported version %d", raw.Version))
	}
	c := &Catalogue{items: raw.Items, byID: map[string]*Def{}}
	for i := range c.items {
		d := &c.items[i]
		if !slugRe.MatchString(d.ID) {
			errs = append(errs, fmt.Errorf("item[%d]: invalid id %q", i, d.ID))
		}
		if _, dup := c.byID[d.ID]; dup {
			errs = append(errs, fmt.Errorf("item %q: duplicate id", d.ID))
		}
		if d.Name == "" {
			errs = append(errs, fmt.Errorf("item %q: empty name", d.ID))
		}
		if !categories[d.Category] {
			errs = append(errs, fmt.Errorf("item %q: unknown category %q", d.ID, d.Category))
		}
		if !rarities[d.Rarity] {
			errs = append(errs, fmt.Errorf("item %q: unknown rarity %q", d.ID, d.Rarity))
		}
		if d.OfflineCapBonusHours < 0 || d.OfflineCapBonusHours > 24 {
			errs = append(errs, fmt.Errorf("item %q: offline_cap_bonus_hours out of range", d.ID))
		}
		c.byID[d.ID] = d
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("item: invalid catalogue: %w", errors.Join(errs...))
	}
	return c, nil
}

// Get returns a definition by ID.
func (c *Catalogue) Get(id string) (*Def, bool) { d, ok := c.byID[id]; return d, ok }

// All returns all definitions in file order. Do not mutate.
func (c *Catalogue) All() []Def { return c.items }
