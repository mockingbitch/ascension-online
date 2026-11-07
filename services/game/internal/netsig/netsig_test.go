package netsig

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseKeys(t *testing.T) {
	ks, err := ParseKeys("k2:"+strings.Repeat("a", 32)+", k1:"+strings.Repeat("b", 32), MinSecretLen)
	if err != nil || len(ks) != 2 || ks[0].ID != "k2" {
		t.Fatalf("%v %v", ks, err)
	}
	for _, bad := range []string{"", "k1", "K1:" + strings.Repeat("a", 32), "k1:short", "k1:" + strings.Repeat("a", 32) + ",k1:" + strings.Repeat("b", 32)} {
		if _, err := ParseKeys(bad, MinSecretLen); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}

func TestNormalize(t *testing.T) {
	a, _ := Normalize("203.0.113.7")
	b, _ := Normalize("::ffff:203.0.113.7")
	if !bytes.Equal(a, b) {
		t.Fatal("IPv4-mapped IPv6 must equal IPv4")
	}
	c, _ := Normalize("2001:db8:1:2::1")
	d, _ := Normalize("2001:db8:1:2:aaaa:bbbb:cccc:dddd")
	e, _ := Normalize("2001:db8:1:3::1")
	if !bytes.Equal(c, d) || bytes.Equal(c, e) {
		t.Fatal("IPv6 must compare by /64")
	}
	if _, err := Normalize("not-an-ip"); err == nil {
		t.Fatal("garbage must fail")
	}
}

func TestHashSaltedTruncatedNoRawIP(t *testing.T) {
	h1 := &Hasher{Keys: []Key{{ID: "a", Secret: []byte(strings.Repeat("x", 32))}}}
	h2 := &Hasher{Keys: []Key{{ID: "a", Secret: []byte(strings.Repeat("y", 32))}}}
	x, err := h1.Hash("198.51.100.4")
	if err != nil || len(x) != 1 || len(x[0].Sum) != HashLen {
		t.Fatalf("%v %v", x, err)
	}
	y, _ := h1.Hash("198.51.100.4")
	z, _ := h2.Hash("198.51.100.4")
	if !bytes.Equal(x[0].Sum, y[0].Sum) || bytes.Equal(x[0].Sum, z[0].Sum) {
		t.Fatal("hash must be deterministic per salt and differ across salts")
	}
	if bytes.Contains(x[0].Sum, []byte{198, 51, 100, 4}) || strings.Contains(string(x[0].Sum), "198.51") {
		t.Fatal("raw IP leaked into the hash")
	}
	// Rotation: two keys → one hash per key.
	h3 := &Hasher{Keys: append(h1.Keys, Key{ID: "b", Secret: []byte(strings.Repeat("z", 32))})}
	r, _ := h3.Hash("198.51.100.4")
	if len(r) != 2 || !bytes.Equal(r[0].Sum, x[0].Sum) || r[1].KeyID != "b" {
		t.Fatalf("rotation: %v", r)
	}
}
