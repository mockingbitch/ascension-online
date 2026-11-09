// Package appearance is the character-look catalogue of decisions 2026-11-09 "Ngoại hình…" (GĐ-A) and "Bộ trang phục
// vẽ sẵn…": content/catalog/appearances.yaml — bodies (12, per path) with 3 painted outfits each, hair accessories, robe
// colours and meditation poses — plus the pure rules that turn a stored choice into the look shown for the character's
// CURRENT path.
//
// A look is body + outfit + pose. Hair accessory and robe colour are DERIVED from the outfit (no free mixing: painted
// art cannot follow it). Storage keeps the existing columns: an outfit is stored as its (hair_id, robe_palette_id) pair,
// unique within a body (loader-enforced), so no migration was needed and every stored row stays readable; a stored pair
// that matches no outfit of the shown body (pre-outfit free mixes, a path change) resolves to the body's default outfit.
//
// DISPLAY ONLY: nothing here feeds cultivation, points, breakthrough/tribulation odds, ranking, merit, the daily
// ceiling or điểm duyên. The loader rejects every unknown field, with a dedicated error for effect-like keys
// (item.EffectLikeKey), so a designer cannot attach power to a look.
package appearance

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"time"

	"gopkg.in/yaml.v3"

	"tiendao/game/internal/item"
	"tiendao/game/internal/realm"
)

var (
	slugRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	sexes  = map[string]bool{"nam": true, "nu": true}
	// SVG fallback hairstyles the web knows (CultivatorFigure). Adding one = add its drawing in the same change.
	svgHairs = map[string]bool{"bui_cao": true, "bui_nua": true, "duoi_ngua": true, "van_cao": true, "tet_lech": true,
		"xoa_buoc_thap": true, "bim_nhieu": true, "chom": true, "tet_duoi": true, "ngan_khan": true}
)

const maxCooldown = 30 * 24 * time.Hour // typo guard

// OutfitsPerBody: every body ships exactly this many painted outfits (decision "Bộ trang phục vẽ sẵn": default + 2).
const OutfitsPerBody = 3

var (
	// ErrUnknown: an id that is not in the catalogue (or not of that kind).
	ErrUnknown = errors.New("appearance: unknown look")
	// ErrWrongPath: the option cannot be worn on the character's path.
	ErrWrongPath = errors.New("appearance: not for this path")
	// ErrNotOwned: the option is not free and the character has not unlocked it.
	ErrNotOwned = errors.New("appearance: not owned")
)

// CooldownError: the body changed less than body_change_cooldown_seconds ago.
type CooldownError struct{ RetryAt time.Time }

func (e *CooldownError) Error() string {
	return "appearance: body changed recently, retry at " + e.RetryAt.UTC().Format(time.RFC3339)
}

// Outfit is one painted outfit of a body: a robe colour + a hair accessory baked into its art.
type Outfit struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	RobePaletteID string `json:"robePaletteId"`
	HairID        string `json:"hairId"`
	// Art is the one-line art brief for the image model (designer text, not served).
	Art string `json:"-"`
}

// Body is one selectable figure. Hair style is part of the body (decision #5).
type Body struct {
	ID          string       `json:"id"`
	Path        realm.PathID `json:"path"`
	Sex         string       `json:"sex"`
	Name        string       `json:"name"`
	Temperament string       `json:"temperament"`
	Traits      string       `json:"traits"`
	SVGHair     string       `json:"svgHair"`
	Beard       bool         `json:"beard,omitempty"`
	// DefaultHair / DefaultRobe: the parts of DefaultOutfit (derived, kept for older clients).
	DefaultHair   string   `json:"defaultHair"`
	DefaultRobe   string   `json:"defaultRobe"`
	DefaultOutfit string   `json:"defaultOutfit"`
	DefaultPose   string   `json:"defaultPose"`
	Pair          string   `json:"pair"`
	DefaultFree   bool     `json:"defaultFree"`
	Outfits       []Outfit `json:"outfits"`
}

// Outfit returns the body's outfit by id.
func (b *Body) Outfit(id string) (*Outfit, bool) {
	for i := range b.Outfits {
		if b.Outfits[i].ID == id {
			return &b.Outfits[i], true
		}
	}
	return nil, false
}

// outfitByParts: the body's outfit with exactly this accessory + robe (unique within a body).
func (b *Body) outfitByParts(hair, robe string) (*Outfit, bool) {
	for i := range b.Outfits {
		if o := &b.Outfits[i]; o.HairID == hair && o.RobePaletteID == robe {
			return o, true
		}
	}
	return nil, false
}

func (b *Body) defaultOutfit() *Outfit { o, _ := b.Outfit(b.DefaultOutfit); return o }

// Option is a hair accessory or a robe colour (worn on `Paths`).
type Option struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Paths       []realm.PathID `json:"paths"`
	DefaultFree bool           `json:"defaultFree"`
}

// Pose is a meditation pose (one path).
type Pose struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Path        realm.PathID `json:"path"`
	DefaultFree bool         `json:"defaultFree"`
}

// Settings are catalogue-wide rules.
type Settings struct {
	BodyChangeCooldownSeconds int64 `json:"bodyChangeCooldownSeconds"`
}

// Look is a character's look: ids only (the client resolves names/visuals from the catalogue). As stored, empty =
// not chosen; as served (Resolve), always complete and valid for the character's current path. HairID and
// RobePaletteID are the parts of OutfitID (Normalize / Resolve derive them; a request only needs body, outfit, pose).
type Look struct {
	BodyID        string `json:"bodyId"`
	OutfitID      string `json:"outfitId"`
	HairID        string `json:"hairId"`
	RobePaletteID string `json:"robePaletteId"`
	PoseID        string `json:"poseId"`
}

// Catalogue is immutable once loaded.
type Catalogue struct {
	Settings  Settings `json:"settings"`
	Bodies    []Body   `json:"bodies"`
	Hairs     []Option `json:"hairAccessories"`
	Robes     []Option `json:"robePalettes"`
	Poses     []Pose   `json:"poses"`
	bodies    map[string]*Body
	hairs     map[string]*Option
	robes     map[string]*Option
	poses     map[string]*Pose
	defaultOf map[realm.PathID]*Body
}

type fileFile struct {
	Version  int `yaml:"version"`
	Settings struct {
		BodyChangeCooldownSeconds int64          `yaml:"body_change_cooldown_seconds"`
		Extra                     map[string]any `yaml:",inline"`
	} `yaml:"settings"`
	Bodies []struct {
		ID            string         `yaml:"id"`
		Path          realm.PathID   `yaml:"path"`
		Sex           string         `yaml:"sex"`
		Name          string         `yaml:"name"`
		Temperament   string         `yaml:"temperament"`
		Traits        string         `yaml:"traits"`
		SVGHair       string         `yaml:"svg_hair"`
		Beard         bool           `yaml:"beard"`
		DefaultOutfit string         `yaml:"default_outfit"`
		DefaultPose   string         `yaml:"default_pose"`
		Pair          string         `yaml:"pair"`
		DefaultFree   bool           `yaml:"default_free"`
		Outfits       []fileOutfit   `yaml:"outfits"`
		Extra         map[string]any `yaml:",inline"`
	} `yaml:"bodies"`
	Hairs []fileOption `yaml:"hair_accessories"`
	Robes []fileOption `yaml:"robe_palettes"`
	Poses []struct {
		ID          string         `yaml:"id"`
		Name        string         `yaml:"name"`
		Path        realm.PathID   `yaml:"path"`
		DefaultFree bool           `yaml:"default_free"`
		Extra       map[string]any `yaml:",inline"`
	} `yaml:"poses"`
}

type fileOutfit struct {
	ID            string         `yaml:"id"`
	Name          string         `yaml:"name"`
	RobePaletteID string         `yaml:"robe_palette"`
	HairID        string         `yaml:"hair_accessory"`
	Art           string         `yaml:"art"`
	Extra         map[string]any `yaml:",inline"`
}

type fileOption struct {
	ID          string         `yaml:"id"`
	Name        string         `yaml:"name"`
	Paths       []realm.PathID `yaml:"paths"`
	DefaultFree bool           `yaml:"default_free"`
	Extra       map[string]any `yaml:",inline"`
}

func rejectExtra(where string, extra map[string]any, fail func(string, ...any)) {
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if item.EffectLikeKey(k) {
			fail("%s: field %q is a gameplay effect — appearances are display only", where, k)
		} else {
			fail("%s: unknown field %q (appearances are display only)", where, k)
		}
	}
}

// LoadFile parses and validates appearances.yaml.
func LoadFile(path string) (*Catalogue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load parses and validates appearances YAML. Paths are checked against realms.yaml by CrossCheck.
func Load(r io.Reader) (*Catalogue, error) {
	var raw fileFile
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("appearance: decode: %w", err)
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if raw.Version != 1 {
		fail("unsupported version %d", raw.Version)
	}
	rejectExtra("settings", raw.Settings.Extra, fail)
	cd := raw.Settings.BodyChangeCooldownSeconds
	if cd < 0 || time.Duration(cd)*time.Second > maxCooldown {
		fail("settings.body_change_cooldown_seconds must be within 0..%d", int64(maxCooldown/time.Second))
	}
	c := &Catalogue{Settings: Settings{BodyChangeCooldownSeconds: cd}, bodies: map[string]*Body{}, hairs: map[string]*Option{},
		robes: map[string]*Option{}, poses: map[string]*Pose{}, defaultOf: map[realm.PathID]*Body{}}
	free := func(where string, ok bool) {
		if !ok {
			fail("%s: default_free must be true — no unlock source for paid/earned looks yet (decisions 2026-11-09 #2)", where)
		}
	}
	options := func(kind string, in []fileOption, out *[]Option) {
		seen := map[string]bool{}
		for i, o := range in {
			where := fmt.Sprintf("%s[%d] %q", kind, i, o.ID)
			rejectExtra(where, o.Extra, fail)
			if !slugRe.MatchString(o.ID) || seen[o.ID] {
				fail("%s: invalid or duplicate id", where)
			}
			seen[o.ID] = true
			if o.Name == "" {
				fail("%s: empty name", where)
			}
			if len(o.Paths) == 0 {
				fail("%s: paths required", where)
			}
			free(where, o.DefaultFree)
			*out = append(*out, Option{ID: o.ID, Name: o.Name, Paths: o.Paths, DefaultFree: o.DefaultFree})
		}
	}
	options("hair_accessories", raw.Hairs, &c.Hairs)
	options("robe_palettes", raw.Robes, &c.Robes)
	for i, p := range raw.Poses {
		where := fmt.Sprintf("poses[%d] %q", i, p.ID)
		rejectExtra(where, p.Extra, fail)
		if !slugRe.MatchString(p.ID) || slices.ContainsFunc(c.Poses, func(x Pose) bool { return x.ID == p.ID }) {
			fail("%s: invalid or duplicate id", where)
		}
		if p.Name == "" || p.Path == "" {
			fail("%s: name and path required", where)
		}
		free(where, p.DefaultFree)
		c.Poses = append(c.Poses, Pose{ID: p.ID, Name: p.Name, Path: p.Path, DefaultFree: p.DefaultFree})
	}
	for i := range c.Hairs {
		c.hairs[c.Hairs[i].ID] = &c.Hairs[i]
	}
	for i := range c.Robes {
		c.robes[c.Robes[i].ID] = &c.Robes[i]
	}
	for i := range c.Poses {
		c.poses[c.Poses[i].ID] = &c.Poses[i]
	}
	outfitSeen := map[string]bool{}
	for i, b := range raw.Bodies {
		where := fmt.Sprintf("bodies[%d] %q", i, b.ID)
		rejectExtra(where, b.Extra, fail)
		if !slugRe.MatchString(b.ID) || c.bodies[b.ID] != nil {
			fail("%s: invalid or duplicate id", where)
		}
		if b.Name == "" || b.Path == "" {
			fail("%s: name and path required", where)
		}
		if !sexes[b.Sex] {
			fail("%s: sex must be nam or nu", where)
		}
		if !svgHairs[b.SVGHair] {
			fail("%s: unknown svg_hair %q (the web has no SVG drawing for it)", where, b.SVGHair)
		}
		free(where, b.DefaultFree)
		body := Body{ID: b.ID, Path: b.Path, Sex: b.Sex, Name: b.Name, Temperament: b.Temperament, Traits: b.Traits,
			SVGHair: b.SVGHair, Beard: b.Beard, DefaultOutfit: b.DefaultOutfit,
			DefaultPose: b.DefaultPose, Pair: b.Pair, DefaultFree: b.DefaultFree}
		if len(b.Outfits) != OutfitsPerBody {
			fail("%s: %d outfits, want exactly %d (decision \"Bộ trang phục vẽ sẵn\")", where, len(b.Outfits), OutfitsPerBody)
		}
		robesSeen := map[string]bool{}
		for j, o := range b.Outfits {
			ow := fmt.Sprintf("%s outfits[%d] %q", where, j, o.ID)
			rejectExtra(ow, o.Extra, fail)
			if !slugRe.MatchString(o.ID) || outfitSeen[o.ID] {
				fail("%s: invalid or duplicate outfit id (ids are unique across all bodies)", ow)
			}
			outfitSeen[o.ID] = true
			if o.Name == "" || o.Art == "" {
				fail("%s: name and art required", ow)
			}
			if h, ok := c.hairs[o.HairID]; !ok || !slices.Contains(h.Paths, b.Path) {
				fail("%s: hair_accessory %q is not a hair accessory of path %q", ow, o.HairID, b.Path)
			}
			if r, ok := c.robes[o.RobePaletteID]; !ok || !slices.Contains(r.Paths, b.Path) {
				fail("%s: robe_palette %q is not a robe palette of path %q", ow, o.RobePaletteID, b.Path)
			}
			// Distinct robe colours: the stored (hair, robe) pair identifies the outfit, and even the SVG fallback differs.
			if robesSeen[o.RobePaletteID] {
				fail("%s: robe_palette %q used twice in this body", ow, o.RobePaletteID)
			}
			robesSeen[o.RobePaletteID] = true
			body.Outfits = append(body.Outfits, Outfit{ID: o.ID, Name: o.Name, RobePaletteID: o.RobePaletteID, HairID: o.HairID, Art: o.Art})
		}
		if d, ok := body.Outfit(b.DefaultOutfit); !ok {
			fail("%s: default_outfit %q is not one of its outfits", where, b.DefaultOutfit)
		} else {
			body.DefaultHair, body.DefaultRobe = d.HairID, d.RobePaletteID
		}
		if p, ok := c.poses[b.DefaultPose]; !ok || p.Path != b.Path {
			fail("%s: default_pose %q is not a pose of path %q", where, b.DefaultPose, b.Path)
		}
		c.Bodies = append(c.Bodies, body)
		c.bodies[b.ID] = &c.Bodies[len(c.Bodies)-1]
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("appearance: invalid catalogue: %w", errors.Join(errs...))
	}
	// Re-point after the slice stopped growing (append may have moved it).
	for i := range c.Bodies {
		c.bodies[c.Bodies[i].ID] = &c.Bodies[i]
	}
	for i := range c.Bodies {
		b := &c.Bodies[i]
		if _, ok := c.defaultOf[b.Path]; !ok {
			c.defaultOf[b.Path] = b // first body of a path in file order = the path default
		}
		p, ok := c.bodies[b.Pair]
		switch {
		case !ok:
			fail("bodies %q: unknown pair %q", b.ID, b.Pair)
		case p.Path == b.Path:
			fail("bodies %q: pair %q must be on the other path", b.ID, b.Pair)
		case p.Sex != b.Sex:
			fail("bodies %q: pair %q must have the same sex", b.ID, b.Pair)
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("appearance: invalid catalogue: %w", errors.Join(errs...))
	}
	return c, nil
}

// CrossCheck validates the catalogue against realms.yaml: every path used exists, and every playable path offers at
// least one free body of each sex (onboarding must always have a choice).
func (c *Catalogue) CrossCheck(realms *realm.Catalogue) error {
	var errs []error
	check := func(where string, p realm.PathID) {
		if !realms.HasPath(p) {
			errs = append(errs, fmt.Errorf("appearances %s: unknown path %q", where, p))
		}
	}
	for _, b := range c.Bodies {
		check("body "+b.ID, b.Path)
	}
	for _, o := range append(slices.Clone(c.Hairs), c.Robes...) {
		for _, p := range o.Paths {
			check("option "+o.ID, p)
		}
	}
	for _, p := range c.Poses {
		check("pose "+p.ID, p.Path)
	}
	for _, p := range realms.Paths() {
		for sex := range sexes {
			if !slices.ContainsFunc(c.Bodies, func(b Body) bool { return b.Path == p.ID && b.Sex == sex && b.DefaultFree }) {
				errs = append(errs, fmt.Errorf("appearances: path %q has no free %s body", p.ID, sex))
			}
		}
	}
	return errors.Join(errs...)
}

// Body returns a body by id.
func (c *Catalogue) Body(id string) (*Body, bool) { b, ok := c.bodies[id]; return b, ok }

// owns reports whether a character may wear a look option. GĐ-A: every shipped option is free (loader-enforced);
// earned / bought / Tiên Duyên looks plug in here with their unlock source.
func owns(free bool) bool { return free }

func (c *Catalogue) hairOK(id string, p realm.PathID) bool {
	h, ok := c.hairs[id]
	return ok && slices.Contains(h.Paths, p) && owns(h.DefaultFree)
}

func (c *Catalogue) robeOK(id string, p realm.PathID) bool {
	o, ok := c.robes[id]
	return ok && slices.Contains(o.Paths, p) && owns(o.DefaultFree)
}

func (c *Catalogue) poseOK(id string, p realm.PathID) bool {
	o, ok := c.poses[id]
	return ok && o.Path == p && owns(o.DefaultFree)
}

// effectiveBody: the stored body if it is on this path, its pair after a path change, else the path default.
func (c *Catalogue) effectiveBody(stored string, p realm.PathID) *Body {
	if b, ok := c.bodies[stored]; ok {
		if b.Path == p && owns(b.DefaultFree) {
			return b
		}
		if pb, ok := c.bodies[b.Pair]; ok && pb.Path == p {
			return pb
		}
	}
	return c.defaultOf[p]
}

func (c *Catalogue) outfitOK(o *Outfit, p realm.PathID) bool {
	return c.hairOK(o.HairID, p) && c.robeOK(o.RobePaletteID, p)
}

// Resolve turns a stored choice into the look shown for path p: always complete, every id valid for p. The outfit is
// the shown body's outfit named by OutfitID, else the one whose (hair, robe) pair was stored, else the body's default
// outfit (path changed, pre-outfit mixes, retired ids); a pose that cannot be held on p falls back to the default. Pure.
func (c *Catalogue) Resolve(stored Look, p realm.PathID) Look {
	b := c.effectiveBody(stored.BodyID, p)
	if b == nil {
		return Look{}
	}
	o := b.defaultOutfit()
	if x, ok := b.Outfit(stored.OutfitID); ok && c.outfitOK(x, p) {
		o = x
	} else if x, ok := b.outfitByParts(stored.HairID, stored.RobePaletteID); ok && c.outfitOK(x, p) {
		o = x
	}
	out := Look{BodyID: b.ID, OutfitID: o.ID, HairID: o.HairID, RobePaletteID: o.RobePaletteID, PoseID: b.DefaultPose}
	if c.poseOK(stored.PoseID, p) {
		out.PoseID = stored.PoseID
	}
	return out
}

// Normalize puts a requested look in canonical form: with an outfit id, its accessory + robe are filled from the
// outfit (the client never chooses them freely); without one (an older client), the outfit is found from the
// accessory + robe pair. Unknown ids are left as sent for Validate to refuse. Call before Validate and before
// comparing with an idempotent replay. Pure.
func (c *Catalogue) Normalize(l Look) Look {
	b, ok := c.bodies[l.BodyID]
	if !ok {
		return l
	}
	if l.OutfitID == "" {
		if o, ok := b.outfitByParts(l.HairID, l.RobePaletteID); ok {
			l.OutfitID = o.ID
		}
		return l
	}
	if o, ok := b.Outfit(l.OutfitID); ok {
		l.HairID, l.RobePaletteID = o.HairID, o.RobePaletteID
	}
	return l
}

// Validate checks a requested, Normalize-d look for path p (body, outfit of that body, pose all required). Pure.
func (c *Catalogue) Validate(want Look, p realm.PathID) error {
	b, ok := c.bodies[want.BodyID]
	if !ok {
		return fmt.Errorf("%w: body %q", ErrUnknown, want.BodyID)
	}
	if b.Path != p {
		return fmt.Errorf("%w: body %q", ErrWrongPath, want.BodyID)
	}
	if !owns(b.DefaultFree) {
		return fmt.Errorf("%w: body %q", ErrNotOwned, want.BodyID)
	}
	if o, ok := b.Outfit(want.OutfitID); !ok {
		return fmt.Errorf("%w: outfit %q of body %q", ErrUnknown, want.OutfitID, want.BodyID)
	} else if o.HairID != want.HairID || o.RobePaletteID != want.RobePaletteID {
		return fmt.Errorf("%w: outfit %q is %s + %s (hair and robe come from the outfit)", ErrUnknown, o.ID, o.HairID, o.RobePaletteID)
	}
	if h, ok := c.hairs[want.HairID]; !ok {
		return fmt.Errorf("%w: hair accessory %q", ErrUnknown, want.HairID)
	} else if !slices.Contains(h.Paths, p) {
		return fmt.Errorf("%w: hair accessory %q", ErrWrongPath, want.HairID)
	} else if !owns(h.DefaultFree) {
		return fmt.Errorf("%w: hair accessory %q", ErrNotOwned, want.HairID)
	}
	if o, ok := c.robes[want.RobePaletteID]; !ok {
		return fmt.Errorf("%w: robe palette %q", ErrUnknown, want.RobePaletteID)
	} else if !slices.Contains(o.Paths, p) {
		return fmt.Errorf("%w: robe palette %q", ErrWrongPath, want.RobePaletteID)
	} else if !owns(o.DefaultFree) {
		return fmt.Errorf("%w: robe palette %q", ErrNotOwned, want.RobePaletteID)
	}
	if o, ok := c.poses[want.PoseID]; !ok {
		return fmt.Errorf("%w: pose %q", ErrUnknown, want.PoseID)
	} else if o.Path != p {
		return fmt.Errorf("%w: pose %q", ErrWrongPath, want.PoseID)
	} else if !owns(o.DefaultFree) {
		return fmt.Errorf("%w: pose %q", ErrNotOwned, want.PoseID)
	}
	return nil
}

// Change is the outcome of applying a requested look to a stored one (Plan).
type Change struct {
	// Store is what to write: BodyID stays as stored when the requested body is the one already shown, so a pending
	// path-change re-pick is not consumed by an unrelated edit (robe, accessory, pose).
	Store Look
	// BodyChanged: the shown body changes → body_changed_at = now.
	BodyChanged bool
}

// Plan decides an in-game look change (POST /cosmetics/equip). The body may change when
//   - it never changed in game (changedAt nil — the onboarding pick does not start the cooldown), or
//   - the stored body is on another path (path changed: one free re-pick, decision #4), or
//   - the cooldown has passed.
//
// want must already be Validate-d for p. Pure.
func (c *Catalogue) Plan(stored Look, changedAt *time.Time, want Look, p realm.PathID, now time.Time) (Change, error) {
	shown := c.Resolve(stored, p)
	ch := Change{Store: want}
	if want.BodyID == shown.BodyID {
		ch.Store.BodyID = stored.BodyID
		return ch, nil
	}
	sb, storedOK := c.bodies[stored.BodyID]
	repick := storedOK && sb.Path != p
	if changedAt != nil && !repick {
		retry := changedAt.Add(time.Duration(c.Settings.BodyChangeCooldownSeconds) * time.Second)
		if now.Before(retry) {
			return Change{}, &CooldownError{RetryAt: retry}
		}
	}
	ch.BodyChanged = true
	return ch, nil
}

// BodyRetryAt is when the next in-game body change is allowed (nil = now): the same rule as Plan, for display. Pure.
func (c *Catalogue) BodyRetryAt(stored Look, changedAt *time.Time, p realm.PathID, now time.Time) *time.Time {
	if changedAt == nil {
		return nil
	}
	if b, ok := c.bodies[stored.BodyID]; ok && b.Path != p {
		return nil // path changed: one free re-pick
	}
	retry := changedAt.Add(time.Duration(c.Settings.BodyChangeCooldownSeconds) * time.Second)
	if !now.Before(retry) {
		return nil
	}
	return &retry
}
