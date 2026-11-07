// Package payment is the top-up (nạp Tiên Duyên) framework (Phase 4): payment orders, provider adapters behind the
// Provider interface (sandbox now; PayOS/VNPay/MoMo later), signature-verified idempotent webhooks, reconciliation,
// expiry and the admin refund path. The ONLY way an order credits điểm duyên is vip.TopupLocked with the order id as
// the anchor, called from apply() after a verified provider event (webhook or reconciliation query). The browser's
// return URL never credits anything; requests never carry amounts or points the server trusts.
package payment

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"time"

	"gopkg.in/yaml.v3"
)

// Envs a provider may be enabled in (config APP_ENV).
var knownEnvs = map[string]bool{"dev": true, "staging": true, "prod": true}

// adapters are the provider ids that have an implementation in this package. Enabling any other id fails the loader.
var adapters = map[string]bool{ProviderSandbox: true}

// knownProviders may appear (disabled) in payments.yaml.
var knownProviders = map[string]bool{ProviderSandbox: true, "payos": true, "vnpay": true, "momo": true}

var slugRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ProviderDef is one provider entry of payments.yaml.
type ProviderDef struct {
	ID      string   `yaml:"id" json:"id"`
	Name    string   `yaml:"name" json:"name"`
	Enabled bool     `yaml:"enabled" json:"-"`
	Envs    []string `yaml:"envs" json:"-"`
}

// Package is one top-up package. Points is computed from vip.yaml (Bind), never authored.
type Package struct {
	ID        string `yaml:"id" json:"id"`
	Name      string `yaml:"name" json:"name"`
	AmountVND int64  `yaml:"amount_vnd" json:"amountVnd"`
	Points    int64  `yaml:"-" json:"points"`
}

// Catalogue is payments.yaml, validated and immutable after Bind.
type Catalogue struct {
	Version               int           `yaml:"version"`
	Currency              string        `yaml:"currency"`
	MinAmountVND          int64         `yaml:"min_amount_vnd"`
	MaxAmountVND          int64         `yaml:"max_amount_vnd"`
	OrderExpiryMinutes    int64         `yaml:"order_expiry_minutes"`
	MaxOpenOrdersPerUser  int64         `yaml:"max_open_orders_per_user"`
	ReconcileAfterMinutes int64         `yaml:"reconcile_after_minutes"`
	ReconcileEveryMinutes int64         `yaml:"reconcile_every_minutes"`
	ReconcileLateHours    int64         `yaml:"reconcile_late_hours"`
	SweepBatch            int32         `yaml:"sweep_batch"`
	Providers             []ProviderDef `yaml:"providers"`
	Packages              []Package     `yaml:"packages"`

	byID map[string]*Package
}

// OrderExpiry is the lifetime of an unpaid order.
func (c *Catalogue) OrderExpiry() time.Duration {
	return time.Duration(c.OrderExpiryMinutes) * time.Minute
}

// ReconcileAfter is the age from which a pending order is queried at the provider.
func (c *Catalogue) ReconcileAfter() time.Duration {
	return time.Duration(c.ReconcileAfterMinutes) * time.Minute
}

// ReconcileEvery is the minimum gap between two provider queries of one order.
func (c *Catalogue) ReconcileEvery() time.Duration {
	return time.Duration(c.ReconcileEveryMinutes) * time.Minute
}

// ReconcileLate is how long after expiry an order is still queried (late payments are honoured).
func (c *Catalogue) ReconcileLate() time.Duration {
	return time.Duration(c.ReconcileLateHours) * time.Hour
}

// Package returns a package by id.
func (c *Catalogue) Package(id string) (Package, bool) {
	p, ok := c.byID[id]
	if !ok {
		return Package{}, false
	}
	return *p, true
}

// EnabledProviders lists the providers enabled for env, in catalogue order.
func (c *Catalogue) EnabledProviders(env string) []ProviderDef {
	out := []ProviderDef{}
	for _, p := range c.Providers {
		if p.Enabled && slices.Contains(p.Envs, env) {
			out = append(out, p)
		}
	}
	return out
}

// LoadFile parses and validates payments.yaml.
func LoadFile(path string) (*Catalogue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load parses and validates payments YAML (Bind must follow before use).
func Load(r io.Reader) (*Catalogue, error) {
	var c Catalogue
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("payment: decode: %w", err)
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if c.Version != 1 {
		fail("unsupported version %d", c.Version)
	}
	if c.Currency != "VND" {
		fail("currency must be VND")
	}
	if c.MinAmountVND < 1000 || c.MinAmountVND%1000 != 0 || c.MaxAmountVND < c.MinAmountVND || c.MaxAmountVND > 50_000_000 || c.MaxAmountVND%1000 != 0 {
		fail("need 1000 ≤ min_amount_vnd ≤ max_amount_vnd ≤ 50,000,000, both multiples of 1000")
	}
	if c.OrderExpiryMinutes < 5 || c.OrderExpiryMinutes > 24*60 {
		fail("order_expiry_minutes must be within 5..1440")
	}
	if c.MaxOpenOrdersPerUser < 1 || c.MaxOpenOrdersPerUser > 20 {
		fail("max_open_orders_per_user must be within 1..20")
	}
	if c.ReconcileAfterMinutes < 1 || c.ReconcileAfterMinutes > c.OrderExpiryMinutes {
		fail("reconcile_after_minutes must be within 1..order_expiry_minutes")
	}
	if c.ReconcileEveryMinutes < 1 || c.ReconcileEveryMinutes > 24*60 {
		fail("reconcile_every_minutes must be within 1..1440")
	}
	if c.ReconcileLateHours < 0 || c.ReconcileLateHours > 24*14 {
		fail("reconcile_late_hours must be within 0..336")
	}
	if c.SweepBatch < 1 || c.SweepBatch > 1000 {
		fail("sweep_batch must be within 1..1000")
	}
	seenP := map[string]bool{}
	for _, p := range c.Providers {
		switch {
		case !knownProviders[p.ID]:
			fail("provider %q unknown", p.ID)
		case seenP[p.ID]:
			fail("provider %q duplicated", p.ID)
		case p.Name == "":
			fail("provider %q: name required", p.ID)
		case p.Enabled && !adapters[p.ID]:
			fail("provider %q enabled but has no adapter in internal/payment", p.ID)
		}
		seenP[p.ID] = true
		if len(p.Envs) == 0 {
			fail("provider %q: envs required", p.ID)
		}
		for _, e := range p.Envs {
			if !knownEnvs[e] {
				fail("provider %q: unknown env %q", p.ID, e)
			}
		}
		// The simulated gateway credits on a self-signed webhook: never in production.
		if p.ID == ProviderSandbox && slices.Contains(p.Envs, "prod") {
			fail("provider sandbox must not list env prod")
		}
	}
	if len(c.Packages) == 0 {
		fail("at least one package required")
	}
	c.byID = map[string]*Package{}
	var prev int64
	for i := range c.Packages {
		p := &c.Packages[i]
		switch {
		case !slugRe.MatchString(p.ID):
			fail("package id %q must be a slug", p.ID)
		case c.byID[p.ID] != nil:
			fail("package %q duplicated", p.ID)
		case p.Name == "":
			fail("package %q: name required", p.ID)
		case p.AmountVND%1000 != 0 || p.AmountVND < c.MinAmountVND || p.AmountVND > c.MaxAmountVND:
			fail("package %q: amount_vnd %d must be a multiple of 1000 within min..max", p.ID, p.AmountVND)
		case p.AmountVND <= prev:
			fail("package %q: amounts must strictly increase", p.ID)
		}
		prev = max(prev, p.AmountVND)
		c.byID[p.ID] = p
	}
	// Zero enabled providers is legal: the web shows "kênh nạp chưa mở".
	if len(errs) > 0 {
		return nil, fmt.Errorf("payment: invalid catalogue: %w", errors.Join(errs...))
	}
	return &c, nil
}

// Bind computes every package's points with the vip.yaml rate (pointsFor = vip.Table.PointsForVND, the same function
// vip.TopupLocked credits with) and checks the rate converts every allowed amount exactly and within maxDelta.
func (c *Catalogue) Bind(pointsFor func(int64) int64, maxDelta int64) error {
	var errs []error
	for i := range c.Packages {
		p := &c.Packages[i]
		p.Points = pointsFor(p.AmountVND)
		if p.Points <= 0 {
			errs = append(errs, fmt.Errorf("payments package %q: %dđ gives no điểm duyên", p.ID, p.AmountVND))
		}
	}
	if top := pointsFor(c.MaxAmountVND); top > maxDelta {
		errs = append(errs, fmt.Errorf("payments max_amount_vnd gives %d điểm duyên > vip.MaxGrantDelta %d", top, maxDelta))
	}
	if pointsFor(c.MinAmountVND) <= 0 {
		errs = append(errs, fmt.Errorf("payments min_amount_vnd gives no điểm duyên"))
	}
	return errors.Join(errs...)
}
