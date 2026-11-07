package config

import (
	"net/netip"
	"strings"
	"testing"
)

// Review M1: TRUSTED_PROXY_CIDRS parsing, dev default and the requirement outside dev.
func TestTrustedProxies(t *testing.T) {
	dev := Config{Env: "dev", AccessTokenTTL: 900e9}
	ps, err := dev.TrustedProxies()
	if err != nil || !inAny(ps, "172.18.0.3") || !inAny(ps, "127.0.0.1") || inAny(ps, "203.0.113.1") {
		t.Fatalf("dev default: %v %v", ps, err)
	}
	c := Config{Env: "prod", TrustedProxyCIDRs: "10.0.0.2, 192.168.1.0/24"}
	if ps, err = c.TrustedProxies(); err != nil || len(ps) != 2 || !inAny(ps, "10.0.0.2") || inAny(ps, "10.0.0.3") {
		t.Fatalf("explicit: %v %v", ps, err)
	}
	if ps, _ := (Config{Env: "prod"}).TrustedProxies(); len(ps) != 0 {
		t.Fatal("no default outside dev")
	}
	if _, err := (Config{Env: "prod", TrustedProxyCIDRs: "nope"}).TrustedProxies(); err == nil {
		t.Fatal("garbage must fail")
	}
	bad := Config{Env: "prod", TrustProxy: true, AccessTokenTTL: 900e9}
	if err := bad.ValidateAPI(); err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXY_CIDRS") {
		t.Fatalf("validate: %v", err)
	}
}

func inAny(ps []netip.Prefix, ip string) bool {
	a := netip.MustParseAddr(ip)
	for _, p := range ps {
		if p.Contains(a) {
			return true
		}
	}
	return false
}
