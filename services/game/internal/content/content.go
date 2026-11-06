// Package content loads every catalogue file and validates cross-references between them.
// Processes (api, worker, balancesim) load content ONLY through LoadAll.
package content

import (
	"errors"
	"fmt"
	"path/filepath"

	"tiendao/game/internal/contribution"
	"tiendao/game/internal/dailytask"
	"tiendao/game/internal/item"
	"tiendao/game/internal/realm"
	"tiendao/game/internal/spiritroot"
	"tiendao/game/internal/vip"
	"tiendao/game/internal/world"
)

// Bundle is the immutable set of game catalogues.
type Bundle struct {
	Realms       *realm.Catalogue
	VIP          *vip.Table
	Items        *item.Catalogue
	SpiritRoots  *spiritroot.Catalogue
	DailyTasks   *dailytask.Catalogue
	Contribution *contribution.Config
	World        *world.Catalogue
}

// LoadAll loads content/catalog/*.yaml from dir and validates cross-file references.
func LoadAll(dir string) (*Bundle, error) {
	var b Bundle
	var err error
	if b.Realms, err = realm.LoadFile(filepath.Join(dir, "realms.yaml")); err != nil {
		return nil, err
	}
	if b.VIP, err = vip.LoadFile(filepath.Join(dir, "vip.yaml")); err != nil {
		return nil, err
	}
	if b.Items, err = item.LoadFile(filepath.Join(dir, "items.yaml")); err != nil {
		return nil, err
	}
	if b.SpiritRoots, err = spiritroot.LoadFile(filepath.Join(dir, "spirit_roots.yaml")); err != nil {
		return nil, err
	}
	if b.DailyTasks, err = dailytask.LoadFile(filepath.Join(dir, "daily_tasks.yaml")); err != nil {
		return nil, err
	}
	if b.Contribution, err = contribution.LoadFile(filepath.Join(dir, "contribution.yaml")); err != nil {
		return nil, err
	}
	if b.World, err = world.LoadFile(filepath.Join(dir, "world_milestones.yaml")); err != nil {
		return nil, err
	}
	if err := b.crossCheck(); err != nil {
		return nil, err
	}
	return &b, nil
}

func (b *Bundle) crossCheck() error {
	var errs []error
	for _, s := range b.Realms.Stages() {
		for _, req := range s.Breakthrough.Requirements {
			if _, ok := b.Items.Get(req.ItemID); !ok {
				errs = append(errs, fmt.Errorf("stage %q requires unknown item %q", s.ID, req.ItemID))
			}
		}
	}
	// The world counts at most the personal ceiling per character-day.
	if b.World.Settings.WorldCreditCapMilli > b.Contribution.DailyCeilingMilli {
		errs = append(errs, fmt.Errorf("world_credit_cap_points exceeds contribution daily_ceiling_points"))
	}
	// Speed-item cap must be reachable through the idle-free activity bucket only (data-model §4.5).
	if b.Contribution.SpeedItemDailyMilli > b.Contribution.Buckets[contribution.Activity].CapMilli {
		errs = append(errs, fmt.Errorf("speed_items cap exceeds the activity bucket"))
	}
	for _, t := range b.DailyTasks.All() {
		for _, r := range t.Rewards {
			if r.ItemID == "" {
				continue
			}
			if _, ok := b.Items.Get(r.ItemID); !ok {
				errs = append(errs, fmt.Errorf("daily task %q rewards unknown item %q", t.ID, r.ItemID))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("content: cross-reference errors: %w", errors.Join(errs...))
	}
	return nil
}
